package mslog

import (
	"encoding/json"
	"fmt"
	"time"
)

type JsonFormatter struct {
}

var JsonFormat = &JsonFormatter{}

func (j *JsonFormatter) Format(params *LoggerFormatParams) string {
	now := time.Now()
	fields := make(Fields)
	fields["time"] = now.Format("2006/01/02 - 15:04:05")
	fields["level"] = params.Level.Level()
	fields["msg"] = params.Msg
	if params.LoggerFields != nil {
		fields["fields"] = params.LoggerFields
	}
	marshal, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	if params.IsColor {
		levelColor := params.Level.LevelColor()
		return fmt.Sprintf("%s%s%s\n", levelColor, string(marshal), reset)
	}
	return fmt.Sprintf("%s\n", string(marshal))
}
