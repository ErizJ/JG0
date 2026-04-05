package rpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type MsHttpClient struct {
	client     http.Client
	serviceMap map[string]MsService
}

func NewHttpClient() *MsHttpClient {
	client := http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   5,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return &MsHttpClient{
		client:     client,
		serviceMap: map[string]MsService{},
	}
}

func (c *MsHttpClient) Get(url string, args map[string]any) ([]byte, error) {
	if args != nil && len(args) > 0 {
		url = url + "?" + c.toValues(args)
	}
	log.Println(url)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.handleResponse(request)
}

func (c *MsHttpClient) PostForm(url string, args map[string]any) ([]byte, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(c.toValues(args)))
	if err != nil {
		return nil, err
	}
	return c.handleResponse(req)
}

func (c *MsHttpClient) PostJson(url string, args map[string]any) ([]byte, error) {
	jsonStr, _ := json.Marshal(args)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonStr))
	if err != nil {
		return nil, err
	}
	return c.handleResponse(req)
}

func (c *MsHttpClient) toValues(args map[string]any) string {
	if args != nil && len(args) > 0 {
		params := url.Values{}
		for k, v := range args {
			params.Set(k, fmt.Sprintf("%v", v))
		}
		return params.Encode()
	}
	return ""
}

func (c *MsHttpClient) handleResponse(request *http.Request) ([]byte, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("response status is %d", response.StatusCode))
	}
	reader := bufio.NewReader(response.Body)
	buffLen := 79
	buff := make([]byte, buffLen)
	body := make([]byte, 0)
	for {
		n, err := reader.Read(buff)
		if err == io.EOF || n == 0 {
			break
		}
		body = append(body, buff[:n]...)
		if n < buffLen {
			break
		}
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

type HttpConfig struct {
	Protocol string
	Host     string
	Port     int
	Ssl      bool
}

const (
	HTTP  = "HTTP"
	HTTP2 = "HTTP2"
	TCP   = "TCP"
)

const (
	GET      = http.MethodGet
	POSTForm = "POST_FORM"
	POSTJson = "POST_JSON"
)

func (c HttpConfig) Url() string {
	if c.Protocol == "" {
		c.Protocol = HTTP
	}
	switch c.Protocol {
	case HTTP, HTTP2:
		prefix := "http://"
		if c.Ssl {
			prefix = "https://"
		}
		return prefix + c.Host + ":" + strconv.FormatInt(int64(c.Port), 10)
	}
	return ""
}

type MsService interface {
	Env() HttpConfig
}

func (c *MsHttpClient) RegisterHttpService(serviceName string, service MsService) {
	c.serviceMap[serviceName] = service
}

func (c *MsHttpClient) Do(service string, method string) MsService {
	msService, ok := c.serviceMap[service]
	if !ok {
		panic(errors.New("service not found"))
	}
	//找到service里面的Field, 给其中调用的方法赋值
	t := reflect.TypeOf(msService)
	v := reflect.ValueOf(msService)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("service must be pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	fieldFind := -1
	for i := 0; i < tVar.NumField(); i++ {
		name := tVar.Field(i).Name
		if name == method {
			fieldFind = i
			break
		}
	}
	if fieldFind == -1 {
		panic(errors.New("method not found"))
	}
	requestPath := tVar.Field(fieldFind).Tag.Get("msrpc")
	if requestPath == "" {
		panic(errors.New("msrpc tag not exist"))
	}
	split := strings.Split(requestPath, ",")
	mt := split[0]
	path := split[1]
	co := msService.Env()
	prefix := co.Url()
	f := func(args map[string]any) ([]byte, error) {
		fmt.Println(mt, prefix+path)
		if mt == GET {
			return c.Get(prefix+path, args)
		}
		if mt == POSTForm {
			return c.PostForm(prefix+path, args)
		}
		if mt == POSTJson {
			return c.PostJson(prefix+path, args)
		}
		return nil, errors.New("no method match :" + mt)
	}
	value := reflect.ValueOf(f)
	vVar.Field(fieldFind).Set(value)
	return msService
}
