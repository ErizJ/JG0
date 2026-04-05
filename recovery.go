package msgo

import (
	"errors"
	"fmt"
	"github.com/ErizJ/JG0/mserror"
	"net/http"
	"runtime"
	"strings"
)

func Recovery(next HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				if e, ok := err.(error); ok && e != nil {
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
