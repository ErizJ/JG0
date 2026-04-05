package mslog

import (
	"fmt"
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

type TextFormatter struct {
}

var TextFormat = &TextFormatter{}

func (f *TextFormatter) Format(params *LoggerFormatParams) string {
	now := time.Now()
	fieldsStr := ""
	if params.LoggerFields != nil {
		//name=value
		var strBuilder strings.Builder
		length := len(params.LoggerFields)
		for k, v := range params.LoggerFields {
			length--
			format := ""
			if length == 0 {
				format = "%s=%v"
			} else {
				format = "%s=%v,"
			}
			_, err := fmt.Fprintf(&strBuilder, format, k, v)
			if err != nil {
				return "strBuilder err"
			}
		}
		fieldsStr = strBuilder.String()
	}
	if params.IsColor {
		levelColor := params.Level.LevelColor()
		msgColor := params.Level.MsgColor()
		return fmt.Sprintf("%s [msgo] %s %s%v%s | level= %s %s %s | msg=%s %v %s %s \n",
			yellow, reset, blue, now.Format("2006/01/02 - 15:04:05"), reset,
			levelColor, params.Level.Level(), reset, msgColor, params.Msg, reset, fieldsStr,
		)
	}
	return fmt.Sprintf("[msgo] %v | level=%s | msg=%v %s \n",
		now.Format("2006/01/02 - 15:04:05"),
		params.Level.Level(), params.Msg, fieldsStr,
	)
}
