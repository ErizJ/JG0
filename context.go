package msgo

import (
	"errors"
	"fmt"
	"github.com/ErizJ/JG0/binding"
	msLog "github.com/ErizJ/JG0/mslog"
	"github.com/ErizJ/JG0/render"
	"github.com/ErizJ/JG0/validator"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

const defaultMultipartMemory = 32 << 20

type Context struct {
	W                     http.ResponseWriter
	R                     *http.Request
	NodeRouterName        string
	RequestMethod         string
	engine                *Engine
	queryCache            url.Values
	DisallowUnknownFields bool
	IsValidate            bool
	StructValidator       validator.StructValidator
	StatusCode            int
	Logger                *msLog.Logger
	Keys                  map[string]any
	mu                    sync.RWMutex
	sameSite              http.SameSite
}

func (c *Context) SetSameSite(s http.SameSite) {
	c.sameSite = s
}
func (c *Context) Set(key string, data any) {
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]any, 1)
	}
	c.Keys[key] = data
	c.mu.Unlock()
}

func (c *Context) Get(key string) (value any, ok bool) {
	c.mu.RLock()
	value, ok = c.Keys[key]
	c.mu.RUnlock()
	return
}

func (c *Context) BindXML(obj any) error {
	xml := binding.XML
	xml.StructValidator = c.StructValidator
	return c.MustBindWith(obj, xml)
}

func (c *Context) BindJson(obj any) error {
	jsonBinding := binding.JSON
	jsonBinding.DisallowUnknownFields = c.DisallowUnknownFields
	jsonBinding.IsValidate = c.IsValidate
	jsonBinding.StructValidator = c.StructValidator
	return c.MustBindWith(obj, jsonBinding)
}

func (c *Context) MustBindWith(obj any, b binding.Binding) error {
	//如果发生错误，返回400状态码 参数错误
	if err := c.ShouldBindWith(obj, b); err != nil {
		c.W.WriteHeader(http.StatusBadRequest)
		return err
	}
	return nil
}

func (c *Context) ShouldBindWith(obj any, b binding.Binding) error {
	return b.Bind(c.R, obj)
}

func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dstName string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {

		}
	}(src)
	dst, err := os.Create(dstName)
	if err != nil {
		return err
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {

		}
	}(dst)

	_, err = io.Copy(dst, src)
	return err
}

// FormFile 获取文件
func (c *Context) FormFile(key string) (*multipart.FileHeader, error) {
	file, header, err := c.R.FormFile(key)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return header, nil
}

func (c *Context) FormFiles(key string) []*multipart.FileHeader {
	multipartForm, err := c.MultipartForm()
	if err != nil {
		return make([]*multipart.FileHeader, 0)
	}
	return multipartForm.File[key]
}

func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.R.ParseMultipartForm(defaultMultipartMemory)
	return c.R.MultipartForm, err
}

func (c *Context) GetPost(key string) (string, error) {
	if err := c.R.ParseMultipartForm(defaultMultipartMemory); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			return "", err
		}
	}
	return c.R.PostForm.Get(key), nil
}

func (c *Context) GetAllPost() (url.Values, error) {
	if err := c.R.ParseMultipartForm(defaultMultipartMemory); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			return nil, err
		}
	}
	return c.R.PostForm, nil
}

// GetMapQuery http://localhost:8080/queryMap?user[id]=1&user[name]=张三
func (c *Context) GetMapQuery(key string) (map[string]string, bool) {
	c.initQueryCache()
	rs := make(map[string]string)
	exists := false
	for k, v := range c.queryCache {
		start := strings.Index(k, "[")

		if start > 0 && k[:start] == key {
			end := strings.Index(k[start+1:], "]")
			if end > 0 {
				exists = true
				rs[k[start+1:][:end]] = v[0]
			}

		}
	}
	return rs, exists
}

func (c *Context) DefaultQuery(key, defaultValue string) string {
	array, ok := c.GetQueryArray(key)
	if !ok {
		return defaultValue
	}
	return array[0]
}

func (c *Context) GetQuery(key string) string {
	c.initQueryCache()
	return c.queryCache.Get(key)
}

func (c *Context) QueryArray(key string) (values []string) {
	c.initQueryCache()
	values, _ = c.queryCache[key]
	return
}

func (c *Context) GetQueryArray(key string) (values []string, ok bool) {
	c.initQueryCache()
	values, ok = c.queryCache[key]
	return
}

func (c *Context) initQueryCache() {
	if c.queryCache == nil {
		if c.R != nil {
			c.queryCache = c.R.URL.Query()
		} else {
			c.queryCache = url.Values{}
		}
	}
}

func (c *Context) HTMLTemplate(name string, data any, files ...string) error {
	c.W.Header().Set("Content-Type", "text/html;charset=utf-8")
	t := template.New(name)
	parseFiles, err := t.ParseFiles(files...)
	if err != nil {
		return err
	}
	err = parseFiles.Execute(c.W, data)
	return err
}

func (c *Context) HTMLTemplateGlob(name string, data any, pattern string) error {
	c.W.Header().Set("Content-Type", "text/html;charset=utf-8")
	t := template.New(name)
	parseFiles, err := t.ParseGlob(pattern)
	if err != nil {
		return err
	}
	err = parseFiles.Execute(c.W, data)
	return err
}

// HTML 直接输出 html 字符串
func (c *Context) HTML(status int, html string) error {
	return c.Render(&render.HTML{Data: html}, status)
}

// Template 带模板的 html
func (c *Context) Template(name string, data any) error {
	return c.Render(&render.HTML{
		Name:       name,
		Data:       data,
		Template:   c.engine.HTMLRender.Template,
		IsTemplate: true,
	}, http.StatusOK)
}

func (c *Context) JSON(status int, data any) error {
	return c.Render(&render.JSON{Data: data}, status)
}

func (c *Context) XML(status int, data any) error {
	return c.Render(&render.XML{Data: data}, status)
}

// String 字符串
func (c *Context) String(status int, format string, values ...any) error {
	return c.Render(&render.String{
		Format: format,
		Values: values,
	}, status)
}

// Redirect 重定向
func (c *Context) Redirect(status int, location string) error {
	return c.Render(&render.Redirect{
		StatusCode: status,
		Request:    c.R,
		Location:   location,
	}, status)
}

func (c *Context) File(filePath string) {
	http.ServeFile(c.W, c.R, filePath)
}

// FileAttachment 自定义文件名的文件下载
func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		c.W.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	} else {
		c.W.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	c.File(filepath)
}

// FileFromFS 从文件系统获取
func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.R.URL.Path = old
	}(c.R.URL.Path)

	c.R.URL.Path = filepath

	http.FileServer(fs).ServeHTTP(c.W, c.R)
}

// Render 通用渲染
func (c *Context) Render(r render.Render, statusCode int) error {
	c.StatusCode = statusCode
	r.WriteContentType(c.W)
	c.W.WriteHeader(statusCode)
	return r.Render(c.W)
}

func (c *Context) Fail(code int, msg any) {
	_ = c.String(code, fmt.Sprintf("%v", msg))
}

func (c *Context) HandleWithError(statusCode int, obj any, err error) {
	if err != nil {
		if c.engine.errorHandler == nil {
			_ = c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		code, data := c.engine.errorHandler(err)
		err = c.JSON(code, data)
	} else {
		err = c.JSON(statusCode, obj)
	}
}

func (c *Context) SetBasicAuth(username, password string) {
	c.R.Header.Set("Authorization", "Basic "+BasicAuth(username, password))
}

func (c *Context) SetCookie(name, value string, maxAge int, path string, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.W, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		SameSite: c.sameSite,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

func (c *Context) GetCookie(name string) (string, error) {
	cookie, err := c.R.Cookie(name)
	if err != nil {
		return "", err
	}
	val, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return cookie.Value, nil
	}
	return val, nil
}
