package engine

import (
	"errors"
	"fmt"
	"github.com/ErizJ/JG0/mserror"
	"github.com/ErizJ/JG0/mslog"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

type HandlerFunc func(ctx *Context)

type MiddlewareFunc func(handlerFunc HandlerFunc) HandlerFunc

func Logging(next HandlerFunc) HandlerFunc {
	cnf := mslog.LoggerConfig{
		Formatter: mslog.DefaultFormatter,
		Out:       mslog.DefaultWriter,
	}
	return func(ctx *Context) {
		// Start timer
		start := time.Now()
		path := ctx.R.URL.Path
		raw := ctx.R.URL.RawQuery
		//执行业务
		next(ctx)
		// stop timer
		stop := time.Now()
		latency := stop.Sub(start)
		ip, _, _ := net.SplitHostPort(strings.TrimSpace(ctx.R.RemoteAddr))
		clientIP := net.ParseIP(ip)
		if raw != "" {
			path = path + "?" + raw
		}
		param := &mslog.LogFormatterParams{
			Request:    ctx.R,
			TimeStamp:  stop,
			StatusCode: ctx.StatusCode,
			Latency:    latency,
			ClientIP:   clientIP,
			Method:     ctx.R.Method,
			Path:       path,
		}
		_, err := fmt.Fprint(cnf.Out, cnf.Formatter(param))
		if err != nil {
			return
		}
	}
}

func Limiter(next HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		//执行业务
		next(ctx)
	}
}

func Recovery(next HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				if e, ok := err.(error); ok {
					var msError *mserror.MsError
					if errors.As(e, &msError) {
						msError.ExecResult()
						return
					}
				}
				ctx.Logger.Error(strings.Split(traceMsg(err), "\n"))
				ctx.Fail(http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		next(ctx)
	}
}

func traceMsg(err any) string {
	var sb strings.Builder
	var pcs = make([]uintptr, 32)
	n := runtime.Callers(3, pcs)
	sb.WriteString(fmt.Sprintf("%v", err))
	for _, pc := range pcs[:n] {
		//函数
		fn := runtime.FuncForPC(pc)
		file, line := fn.FileLine(pc)
		sb.WriteString(fmt.Sprintf("\n%s:%d", file, line))
	}
	return sb.String()
}
