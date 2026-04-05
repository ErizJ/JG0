package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/ErizJ/JG0/mslog"
	"log"
	"reflect"
	"strings"
	"time"
)

type MsDb struct {
	db     *sql.DB
	logger *mslog.Logger
	Prefix string
}

type MsSession struct {
	db           *MsDb
	TableName    string
	fieldName    []string
	placeHolder  []string
	values       []any
	updateParam  strings.Builder
	whereParam   strings.Builder
	updateValues []any
	beginTx      bool
	tx           *sql.Tx
}

func (s *MsSession) Table(name string) *MsSession {
	s.TableName = name
	return s
}
func (s *MsSession) Count() (int64, error) {
	query := fmt.Sprintf("select count(*) from %s ", s.TableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var result int64
	row := stmt.QueryRow()
	err = row.Err()
	if err != nil {
		return 0, err
	}
	err = row.Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *MsSession) Where(field string, data any) *MsSession {
	if s.whereParam.String() != "" {
		s.whereParam.WriteString(" and ")
	} else {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" = ?")
	s.values = append(s.values, data)
	return s
}

func (s *MsSession) Or(field string, data any) *MsSession {
	if s.whereParam.String() != "" {
		s.whereParam.WriteString(" or ")
	} else {
		s.whereParam.WriteString(" where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" = ?")
	s.values = append(s.values, data)
	return s
}

func (s *MsSession) QueryRow(sql string, data any, queryValues ...any) error {
	t := reflect.TypeOf(data)
	stmt, err := s.db.db.Prepare(sql)
	if err != nil {
		return err
	}
	defer stmt.Close()
	rows, err := stmt.Query(queryValues...)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([]any, len(columns))
	var fieldsScan = make([]any, len(columns))
	for i := range fieldsScan {
		fieldsScan[i] = &values[i]
	}
	if rows.Next() {
		err = rows.Scan(fieldsScan...)
		if err != nil {
			return err
		}
		v := reflect.ValueOf(data)
		valueOf := reflect.ValueOf(values)
		for i := 0; i < t.Elem().NumField(); i++ {
			name := t.Elem().Field(i).Name
			tag := t.Elem().Field(i).Tag
			sqlTag := tag.Get("msorm")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(name))
			} else {
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}
			}
			for j, coName := range columns {
				if sqlTag == coName {
					if v.Elem().Field(i).CanSet() {
						covertValue := s.ConvertType(valueOf, j, v, i)
						v.Elem().Field(i).Set(covertValue)
					}
				}
			}
		}
	}

	return nil

}

func (s *MsSession) Exec(sql string, values ...any) (int64, error) {
	stmt, err := s.db.db.Prepare(sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	r, err := stmt.Exec(values...)
	if err != nil {
		return 0, err
	}
	if strings.Contains(strings.ToLower(sql), "insert") {
		return r.LastInsertId()
	}
	return r.RowsAffected()
}

func (s *MsSession) Update(data ...any) (int64, error) {
	size := len(data)
	if size <= 0 || size > 2 {
		return -1, errors.New("params error")
	}
	single := true
	if size == 2 {
		single = false
	}
	if !single {
		if s.updateParam.String() != "" {
			s.updateParam.WriteString(",")
		}
		field := data[0].(string)
		s.updateParam.WriteString(field)
		s.updateParam.WriteString(" = ?")
		s.updateValues = append(s.updateValues, data[1])
	} else {
		d := data[0]
		t := reflect.TypeOf(d)
		v := reflect.ValueOf(d)
		if t.Kind() != reflect.Pointer {
			return -1, errors.New("data not pointer")
		}
		tVar := t.Elem()
		vVar := v.Elem()
		if s.TableName == "" {
			s.TableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
		}
		for i := 0; i < tVar.NumField(); i++ {
			if s.updateParam.String() != "" {
				s.updateParam.WriteString(",")
			}
			sqlTag := tVar.Field(i).Tag.Get("mssql")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(tVar.Field(i).Name))
			}
			if strings.Contains(sqlTag, ",") {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
			}
			fieldValue := vVar.Field(i).Interface()
			if sqlTag == "id" && isAutoId(fieldValue) {
				continue
			}
			s.updateParam.WriteString(sqlTag)
			s.updateParam.WriteString(" = ?")
			s.updateValues = append(s.updateValues, fieldValue)
		}
	}
	query := fmt.Sprintf("update %s set %s %s", s.TableName, s.updateParam.String(), s.whereParam.String())
	log.Println(query)
	stmt, err := s.db.db.Prepare(query)
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	s.updateValues = append(s.updateValues, s.values...)
	r, err := stmt.Exec(s.updateValues...)
	if err != nil {
		return -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return -1, err
	}

	return affected, nil
}

func (s *MsSession) Insert(data any) (int64, int64, error) {
	s.fieldNames(data)
	query := fmt.Sprintf("insert into %s (%s) values(%s)", s.TableName, strings.Join(s.fieldName, ","), strings.Join(s.placeHolder, ","))
	stmt, err := s.db.db.Prepare(query)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	defer stmt.Close()
	}
	r, err := stmt.Exec(s.values...)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	return id, affected, nil
}

func (s *MsSession) fieldNames(data any) {
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("data type must be pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	if s.TableName == "" {
		s.TableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}

	var fieldNames []string
	var placeholder []string
	var values []any
	for i := 0; i < tVar.NumField(); i++ {
		//首字母是小写的
		if !vVar.Field(i).CanInterface() {
			continue
		}
		//解析tag
		field := tVar.Field(i)
		sqlTag := field.Tag.Get("mssql")
		if sqlTag == "" {
			sqlTag = strings.ToLower(Name(field.Name))
		}
		contains := strings.Contains(sqlTag, "auto_increment")
		if sqlTag == "id" || contains {
			//对id做个判断 如果其值小于等于0 数据库可能是自增 跳过此字段
			if isAutoId(vVar.Field(i).Interface()) {
				continue
			}
		}
		if contains {
			sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
		}
		fieldNames = append(fieldNames, sqlTag)
		placeholder = append(placeholder, "?")
		values = append(values, vVar.Field(i).Interface())
	}
	s.fieldName = fieldNames
	s.placeHolder = placeholder
	s.values = values
}

func (s *MsSession) InsertBath(data []any) (int64, int64, error) {
	if len(data) == 0 {
		panic(errors.New("data type must be slice and not empty"))
	}
	s.batchFieldNames(data)
	// insert into table (*,*) values (?,?),(?,?),(?,?)
	query := fmt.Sprintf("insert into %s (%s) values %s", s.TableName, strings.Join(s.fieldName, ","), strings.Join(s.placeHolder, ","))

	stmt, err := s.db.db.Prepare(query)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	defer stmt.Close()
	}

	r, err := stmt.Exec(s.values...)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	return id, affected, nil
}

func (s *MsSession) batchFieldNames(data []any) {
	var fieldNames []string
	var placeholder []string
	var values []any
	for index, datum := range data {
		t := reflect.TypeOf(datum)
		v := reflect.ValueOf(datum)
		if t.Kind() != reflect.Pointer {
			panic(errors.New("data type must be pointer"))
		}
		tVar := t.Elem()
		vVar := v.Elem()
		if s.TableName == "" {
			s.TableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
		}
		var placeholderArr []string
		for i := 0; i < tVar.NumField(); i++ {
			//首字母是小写的
			if !vVar.Field(i).CanInterface() {
				continue
			}
			//解析tag
			field := tVar.Field(i)
			sqlTag := field.Tag.Get("mssql")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(field.Name))
			}
			contains := strings.Contains(sqlTag, "auto_increment")
			if sqlTag == "id" || contains {
				//对id做个判断 如果其值小于等于0 数据库可能是自增 跳过此字段
				if isAutoId(vVar.Field(i).Interface()) {
					continue
				}
			}
			if contains {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
			}
			if index == 0 {
				fieldNames = append(fieldNames, sqlTag)
			}
			placeholderArr = append(placeholderArr, "?")
			values = append(values, vVar.Field(i).Interface())
			log.Println(vVar.Field(i).Interface())
		}
		placeholder = append(placeholder, "("+strings.Join(placeholderArr, ",")+")")
	}
	s.fieldName = fieldNames
	s.placeHolder = placeholder
	s.values = values
}

func (s *MsSession) Select(data any, fields ...string) ([]any, error) {
	var fieldStr = "*"
	if len(fields) > 0 {
		fieldStr = strings.Join(fields, ",")
	}
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("data type must be struct"))
	}
	if s.TableName == "" {
		s.TableName = s.db.Prefix + strings.ToLower(Name(t.Elem().Name()))
	}
	query := fmt.Sprintf("select %s from %s ", fieldStr, s.TableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(s.values...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]any, len(columns))
	var fieldsScan = make([]any, len(columns))
	for i := range fieldsScan {
		fieldsScan[i] = &values[i]
	}
	var results []any
	for {
		if rows.Next() {
			data = reflect.New(t.Elem()).Interface()
			err = rows.Scan(fieldsScan...)
			if err != nil {
				return nil, err
			}
			v := reflect.ValueOf(data)
			valueOf := reflect.ValueOf(values)
			for i := 0; i < t.Elem().NumField(); i++ {
				name := t.Elem().Field(i).Name
				tag := t.Elem().Field(i).Tag
				sqlTag := tag.Get("msorm")
				if sqlTag == "" {
					sqlTag = strings.ToLower(Name(name))
				} else {
					if strings.Contains(sqlTag, ",") {
						sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
					}
				}
				for j, coName := range columns {
					if sqlTag == coName {
						if v.Elem().Field(i).CanSet() {
							eVar := valueOf.Index(j)
							t2 := v.Elem().Field(i).Type()
							of := reflect.ValueOf(eVar.Interface())
							covertValue := of.Convert(t2)
							v.Elem().Field(i).Set(covertValue)
						}
					}
				}
			}

			results = append(results, data)
		} else {
			break
		}
	}
	return results, nil
}

func (s *MsSession) Delete() error {
	query := fmt.Sprintf("delete from %s ", s.TableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(s.values...)
	if err != nil {
		return err
	}
	return nil
}

func (s *MsSession) ConvertType(valueOf reflect.Value, j int, v reflect.Value, i int) reflect.Value {
	eVar := valueOf.Index(j)
	t2 := v.Elem().Field(i).Type()
	of := reflect.ValueOf(eVar.Interface())
	covertValue := of.Convert(t2)
	return covertValue
}

func Name(name string) string {
	all := name[:]
	var sb strings.Builder
	lastIndex := 0
	for index, value := range all {
		if value >= 65 && value <= 90 {
			if index == 0 {
				continue
			}
			sb.WriteString(name[lastIndex:index])
			sb.WriteString("_")
			lastIndex = index
		}
	}
	sb.WriteString(name[lastIndex:])
	return sb.String()
}

func isAutoId(id any) bool {
	t := reflect.TypeOf(id)
	v := reflect.ValueOf(id)
	switch t.Kind() {
	case reflect.Int64:
		if v.Interface().(int64) <= 0 {
			return true
		}
	case reflect.Int32:
		if v.Interface().(int32) <= 0 {
			return true
		}
	case reflect.Int:
		if v.Interface().(int) <= 0 {
			return true
		}
	default:
		return false
	}
	return false
}

func (d *MsDb) TablePrefix(prefix string) *MsDb {
	d.Prefix = prefix
	return d
}

func (d *MsDb) New() *MsSession {
	return &MsSession{
		db: d,
	}
}

func Open(driverName string, source string) (*MsDb, error) {
	db, err := sql.Open(driverName, source)
	if err != nil {
		panic(err)
	}
	msDb := &MsDb{
		db: db,
	}
	//最大空闲连接数，默认不配置，是2个最大空闲连接
	db.SetMaxIdleConns(5)
	//最大连接数，默认不配置，是不限制最大连接数
	db.SetMaxOpenConns(100)
	// 连接最大存活时间
	db.SetConnMaxLifetime(time.Minute * 3)
	//空闲连接最大存活时间
	db.SetConnMaxIdleTime(time.Minute * 1)
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	return msDb, nil
}

func (s *MsSession) Begin() error {
	tx, err := s.db.db.Begin()
	if err != nil {
		return err
	}
	s.tx = tx
	s.beginTx = true
	return nil
}

func (s *MsSession) Commit() error {
	err := s.tx.Commit()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}

func (s *MsSession) Rollback() error {
	err := s.tx.Rollback()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}
