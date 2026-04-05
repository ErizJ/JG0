package engine

import (
	"fmt"
	"github.com/ErizJ/JG0/engine/gateway"
	"github.com/ErizJ/JG0/mslog"
	"github.com/ErizJ/JG0/register"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Engine struct {
	router
	Logger       *mslog.Logger
	middlewares  []MiddlewareFunc
	errorHandler ErrorHandler
	register     register.MsRegister
}

type ErrorHandler func(err error) (int, any)

func (e *Engine) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := &Context{
		W:      writer,
		R:      request,
		Logger: e.Logger,
	}
	method := ctx.R.Method
	requestPath := ctx.R.URL.Path

	if node := e.node.Get(requestPath); node != nil {
		//网关的处理逻辑
		if e.gatewayConfigs != nil {
			gwConfig, ok := e.gatewayConfigMap[node.routerName]
			if ok {
				var rawURL string
				if e.register == nil {
					rawURL = fmt.Sprintf("http://%s:%d%s", gwConfig.Host, gwConfig.Port, requestPath)
				} else {
					serviceName, err := e.register.GetService(gwConfig.ServiceName)
					e.Logger.Info("注册中心结果")
					if err != nil {

					}
					rawURL = fmt.Sprintf("http://%s", serviceName)
				}
				target, _ := url.Parse(rawURL)
				director := func(request *http.Request) {
					request.Host = target.Host
					request.URL.Host = target.Host
					request.URL.Path = target.Path
					request.URL.Scheme = target.Scheme
					if _, ok := request.Header["User-Agent"]; !ok {
						request.Header.Set("User-Agent", "")
					}
				}
				response := func(response *http.Response) error {
					e.Logger.Info("结果处理")
					return nil
				}
				handler := func(writer http.ResponseWriter, request *http.Request, err error) {
					e.Logger.Info("错误处理")
					e.Logger.Error(err)
				}
				proxy := httputil.ReverseProxy{Director: director, ModifyResponse: response, ErrorHandler: handler}
				proxy.ServeHTTP(writer, request)
				return
			}
		}

		handlerFuncMap, ok := e.handlerFuncMap[node.routerName]
		if ok {
			ctx.NodeRouterName = node.routerName
			for _, v := range []string{method, ANY} {
				if handle, ok := handlerFuncMap[v]; ok {
					ctx.RequestMethod = v
					e.methodHandler(handle, ctx)
					return
				}
			}
			ctx.W.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
	ctx.W.WriteHeader(http.StatusNotFound)
}

func (e *Engine) methodHandler(h HandlerFunc, ctx *Context) {
	//包裹引擎级别中间件
	for _, middleware := range e.middlewares {
		h = middleware(h)
	}

	//包裹方法级别中间件
	funcMiddles := e.router.middlewaresFuncMap[ctx.NodeRouterName][ctx.RequestMethod]
	if funcMiddles != nil {
		funcLen := len(funcMiddles) - 1
		for i := funcLen; i > -1; i-- {
			middleware := funcMiddles[i]
			h = middleware(h)
		}
	}
	//执行方法
	h(ctx)
}

// Run TLS use []string{certFile, keyFile}
func (e *Engine) Run(addr string, file ...string) {
	var err error
	if len(file) == 2 {
		err = http.ListenAndServeTLS(addr, file[0], file[1], e)
	} else {
		http.Handle("/", e)
		err = http.ListenAndServe(addr, nil)
	}

	if err != nil {
		e.Logger.Error(err)
	}
}

func (e *Engine) Use(middlewareFunc ...MiddlewareFunc) {
	e.middlewares = append(e.middlewares, middlewareFunc...)
}

func (e *Engine) SetGateConfigs(configs []gateway.GWConfig) {
	e.gatewayConfigs = configs
	if len(configs) > 0 {
		for _, config := range configs {
			e.node.Put(config.Path, config.Name)
			e.gatewayConfigMap[config.Name] = config
		}
	}
}

func (e *Engine) SetRegister(register register.MsRegister) {
	e.register = register
}

func Default() *Engine {
	engine := New()
	engine.Logger = mslog.Default()

	return engine
}

func New() *Engine {
	r := router{}
	r.handlerFuncMap = make(map[string]map[string]HandlerFunc, 10)
	r.middlewaresFuncMap = make(map[string]map[string][]MiddlewareFunc, 10)
	r.gatewayConfigMap = make(map[string]gateway.GWConfig, 10)
	engine := &Engine{
		router: r,
	}
	return engine
}
