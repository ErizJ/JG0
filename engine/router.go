package engine

import (
	"github.com/ErizJ/JG0/engine/gateway"
	"net/http"
)

type router struct {
	name               string
	handlerFuncMap     map[string]map[string]HandlerFunc
	node               treeNode
	middlewaresFuncMap map[string]map[string][]MiddlewareFunc
	gatewayConfigs     []gateway.GWConfig
	gatewayConfigMap   map[string]gateway.GWConfig
}

const ANY = "ANY"

func (r *router) Any(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, ANY, middlewareFunc...)
}

func (r *router) Get(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.request(name, handlerFunc, http.MethodGet, middlewareFunc...)
}

func (r *router) request(name string, handlerFunc HandlerFunc, method string, middlewareFunc ...MiddlewareFunc) {
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
	r.middlewaresFuncMap[name][method] = append(r.middlewaresFuncMap[name][method], middlewareFunc...)
	r.node.Put(name)
}
