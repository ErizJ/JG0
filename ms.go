package msgo

import (
	"github.com/ErizJ/JG0/config"
	"github.com/ErizJ/JG0/mslog"
	"github.com/ErizJ/JG0/render"
	"html/template"
	"log"
	"net/http"
	"sync"
)

const ANY = "ANY"

type HandlerFunc func(ctx *Context)

type MiddlewareFunc func(handlerFunc HandlerFunc) HandlerFunc

type routerGroup struct {
	name               string
	handlerFuncMap     map[string]map[string]HandlerFunc
	handlerMethodMap   map[string][]string
	treeNode           *treeNode
	middlewares        []MiddlewareFunc
	middlewaresFuncMap map[string]map[string][]MiddlewareFunc
}

func (r *routerGroup) Use(middlewareFunc ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, middlewareFunc...)
}

func (r *routerGroup) methodHandler(h HandlerFunc, ctx *Context) {
	//执行方法前置中间件
	if r.middlewares != nil {
		for _, middleware := range r.middlewares {
			h = middleware(h)
		}
	}
	//执行方法级别的中间件
	funcMiddles := r.middlewaresFuncMap[ctx.NodeRouterName][ctx.RequestMethod]
	if funcMiddles != nil {
		for _, middleware := range funcMiddles {
			h = middleware(h)
		}
	}
	//执行主业务程序
	h(ctx)
}

func (r *routerGroup) request(name string, handlerFunc HandlerFunc, method string, middlewareFunc ...MiddlewareFunc) {
	_, ok := r.handlerFuncMap[name]
	if !ok {
		r.handlerFuncMap[name] = make(map[string]HandlerFunc)
		r.middlewaresFuncMap[name] = make(map[string][]MiddlewareFunc)
	}
	_, ok = r.handlerFuncMap[name][method]
	if ok {
		panic(name + " 有重复的路由")
	}
	r.handlerFuncMap[name][method] = handlerFunc
	r.handlerMethodMap[method] = append(r.handlerMethodMap[method], name)
	r.middlewaresFuncMap[name][method] = append(r.middlewaresFuncMap[name][method], middlewareFunc...)
	r.treeNode.Put(name)
}

func (r *routerGroup) Any(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, ANY, middlewareFunc...)
}

func (r *routerGroup) Get(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodGet, middlewareFunc...)
}

func (r *routerGroup) Post(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodPost, middlewareFunc...)
}

func (r *routerGroup) Put(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodPut, middlewareFunc...)
}

func (r *routerGroup) Delete(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodDelete, middlewareFunc...)
}

func (r *routerGroup) Patch(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodPatch, middlewareFunc...)
}
func (r *routerGroup) Head(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodHead, middlewareFunc...)
}
func (r *routerGroup) Options(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodOptions, middlewareFunc...)
}

type router struct {
	routerGroups []*routerGroup
	engine       *Engine
}

func (r *router) Group(name string) *routerGroup {
	g := &routerGroup{
		name:               name,
		handlerFuncMap:     make(map[string]map[string]HandlerFunc),
		middlewaresFuncMap: make(map[string]map[string][]MiddlewareFunc),
		handlerMethodMap:   make(map[string][]string),
		treeNode:           &treeNode{name: "/", children: make([]*treeNode, 0)},
	}
	g.Use(r.engine.middlewares...)
	r.routerGroups = append(r.routerGroups, g)
	return g
}

type ErrorHandler func(err error) (int, any)

type Engine struct {
	router
	funcMap      template.FuncMap
	HTMLRender   render.HTMLRender
	pool         sync.Pool
	Logger       *mslog.Logger
	middlewares  []MiddlewareFunc
	errorHandler ErrorHandler
}

func New() *Engine {
	engine := &Engine{
		router: router{},
	}
	engine.router.engine = engine
	engine.pool.New = func() any {
		return engine.allocateContext()
	}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Logger = mslog.Default()
	logPath, ok := config.Conf.Log["path"]
	if ok {
		if s, ok := logPath.(string); ok {
			engine.Logger.SetLogPath(s)
		}
	}
	engine.Use(Logging, Recovery)
	engine.router.engine = engine
	return engine
}

func (e *Engine) allocateContext() any {
	return &Context{engine: e}
}

func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	e.funcMap = funcMap
}

func (e *Engine) SetHTMLRender(render render.HTMLRender) {
	e.HTMLRender = render
}

func (e *Engine) LoadTemplate(pattern string) {
	t := template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
	e.SetHTMLRender(render.HTMLRender{Template: t})
}

// 实现 http.server 的 Handler 接口
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := e.pool.Get().(*Context)
	ctx.W = w
	ctx.R = r
	ctx.Logger = e.Logger
	ctx.Keys = nil
	ctx.queryCache = nil
	ctx.StatusCode = 0
	ctx.NodeRouterName = ""
	ctx.RequestMethod = ""
	e.httpRequestHandle(ctx)
	e.pool.Put(ctx)
}

func (e *Engine) RunTLS(addr, certFile, keyFile string) {
	err := http.ListenAndServeTLS(addr, certFile, keyFile, e.Handler())
	if err != nil {
		log.Fatal("ListenAndServeTLS err", err)
	}
}

func (e *Engine) Run(addr string) {

	http.Handle("/", e)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("启动失败", err)
	}
}

func (e *Engine) httpRequestHandle(ctx *Context) {
	method := ctx.R.Method
	for _, group := range e.routerGroups {
		//去掉uri的分组名称
		routerName := SubStringLast(ctx.R.URL.Path, "/"+group.name)
		//路由是否存在
		node := group.treeNode.Get(routerName)
		if node != nil && node.isEnd {
			handlerFunc, ok := group.handlerFuncMap[node.routerName]
			//mslog.Printf("handlerFuncMap [%s] match [%s] %v", routerName, node.routerName, ok)
			if ok {
				ctx.NodeRouterName = node.routerName
				if handle, ok := handlerFunc[method]; ok {
					ctx.RequestMethod = method
					group.methodHandler(handle, ctx)
					return
				}

				if handle, ok := handlerFunc[ANY]; ok {
					ctx.RequestMethod = ANY
					group.methodHandler(handle, ctx)
					return
				}
				ctx.W.WriteHeader(http.StatusMethodNotAllowed)
				log.Printf("%s %s not allowed", ctx.R.RequestURI, method)
				return
			}
		}
	}
	ctx.W.WriteHeader(http.StatusNotFound)
	log.Printf("%s %s not found", ctx.R.RequestURI, method)
}

func (e *Engine) Use(middlewareFunc ...MiddlewareFunc) {
	e.middlewares = append(e.middlewares, middlewareFunc...)
}

func (e *Engine) RegisterErrorHandler(handler ErrorHandler) {
	e.errorHandler = handler
}

func (e *Engine) Handler() http.Handler {
	return e
}
