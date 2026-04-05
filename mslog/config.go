package mslog

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type LoggerFormatter func(params *LogFormatterParams) string

type LogFormatterParams struct {
	Request    *http.Request
	TimeStamp  time.Time
	StatusCode int
	Latency    time.Duration
	ClientIP   net.IP
	Method     string
	Path       string
}

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

type LoggerConfig struct {
	Formatter LoggerFormatter
	Out       io.Writer
}

var DefaultFormatter = func(params *LogFormatterParams) string {
	StatusCodeColor := params.StatusCodeColor(params.StatusCode)
	resetColor := params.ResetColor()
	return fmt.Sprintf("%s [msgo] %s %s%v%s |%s %3d %s| %13v | %15s |%-7s %#v\n",
		yellow, resetColor, blue, params.TimeStamp.Format("2006/01/02 - 15:04:05"), resetColor,
		StatusCodeColor,
		params.StatusCode,
		resetColor,
		params.Latency,
		params.ClientIP,
		params.Method,
		params.Path,
	)
}
var DefaultWriter io.Writer = os.Stdout
