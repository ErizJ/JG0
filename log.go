package msgo

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	greenBg   = "\033[97;42m"
	whiteBg   = "\033[90;47m"
	yellowBg  = "\033[90;43m"
	redBg     = "\033[97;41m"
	blueBg    = "\033[97;44m"
	magentaBg = "\033[97;45m"
	cyanBg    = "\033[97;46m"
	green     = "\033[32m"
	white     = "\033[37m"
	yellow    = "\033[33m"
	red       = "\033[31m"
	blue      = "\033[34m"
	magenta   = "\033[35m"
	cyan      = "\033[36m"
	reset     = "\033[0m"
)

type LoggerConfig struct {
	Formatter LoggerFormatter
	out       io.Writer
}

type LoggerFormatter func(params *LogFormatterParams) string

func (l *LogFormatterParams) StatusCodeColor(StatusCode int) string {
	color := white
	if StatusCode == http.StatusOK {
		color = green
	}
	return color
}

func (l *LogFormatterParams) ResetColor() string {
	return reset
}

type LogFormatterParams struct {
	Request    *http.Request
	TimeStamp  time.Time
	StatusCode int
	Latency    time.Duration
	ClientIP   net.IP
	Method     string
	Path       string
}

var defaultFormatter = func(params *LogFormatterParams) string {
	StatusCodeColor := params.StatusCodeColor(params.StatusCode)
	resetColor := params.ResetColor()
	return fmt.Sprintf("[msgo] %v |%s %3d %s| %13v | %15s |%-7s %#v\n",
		params.TimeStamp.Format("2006/01/02 - 15:04:05"),
		StatusCodeColor,
		params.StatusCode,
		resetColor,
		params.Latency,
		params.ClientIP,
		params.Method,
		params.Path,
	)
}
var defaultWriter io.Writer = os.Stdout

func LoggerWithConfig(cnf LoggerConfig, next HandlerFunc) HandlerFunc {
	if cnf.Formatter == nil {
		cnf.Formatter = defaultFormatter
	}
	if cnf.out == nil {
		cnf.out = defaultWriter
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
		param := &LogFormatterParams{
			Request:    ctx.R,
			TimeStamp:  stop,
			StatusCode: ctx.StatusCode,
			Latency:    latency,
			ClientIP:   clientIP,
			Method:     ctx.R.Method,
			Path:       path,
		}
		_, err := fmt.Fprint(cnf.out, cnf.Formatter(param))
		if err != nil {
			return
		}
	}
}

func Logging(next HandlerFunc) HandlerFunc {
	return LoggerWithConfig(LoggerConfig{}, next)
}
