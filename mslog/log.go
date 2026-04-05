package mslog

import (
	"fmt"
	"github.com/ErizJ/JG0/internal/msstrings"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

type LoggerLevel int

func (l LoggerLevel) Level() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	default:
		return ""
	}
}

func (l LoggerLevel) LevelColor() string {
	switch l {
	case LevelDebug:
		return blue
	case LevelInfo:
		return green
	case LevelError:
		return red
	default:
		return cyan
	}
}

func (l LoggerLevel) MsgColor() string {
	switch l {
	case LevelDebug:
		return ""
	case LevelInfo:
		return ""
	case LevelError:
		return red
	default:
		return cyan
	}
}

const (
	LevelDebug LoggerLevel = iota
	LevelInfo
	LevelError
)

type Fields map[string]any

type LoggerFormat interface {
	Format(*LoggerFormatParams) string
}
type LoggerFormatParams struct {
	Level        LoggerLevel
	IsColor      bool
	LoggerFields Fields
	Msg          any
}

type Logger struct {
	Formatter    LoggerFormat
	Level        LoggerLevel
	Outs         []*LoggerWriter
	LoggerFields Fields
	LogPath      string
	LogFilesize  int64
}
type LoggerWriter struct {
	Level LoggerLevel
	out   io.Writer
}

func New() *Logger {
	return &Logger{}
}

func (l *Logger) Info(msg any) {
	l.Print(LevelInfo, msg)
}

func (l *Logger) Error(msg any) {
	l.Print(LevelError, msg)
}

func (l *Logger) Debug(msg any) {
	l.Print(LevelDebug, msg)
}

func (l *Logger) WithFields(fields Fields) *Logger {
	return &Logger{
		Formatter:    l.Formatter,
		Outs:         l.Outs,
		Level:        l.Level,
		LoggerFields: fields,
	}
}

func (l *Logger) Print(level LoggerLevel, msg any) {
	if level < l.Level {
		return
	}
	params := &LoggerFormatParams{
		Level:        level,
		LoggerFields: l.LoggerFields,
		Msg:          msg,
	}
	for _, out := range l.Outs {
		params.IsColor = out.out == os.Stdout
		if level == out.Level || out.Level == -1 || params.IsColor {
			_, err := fmt.Fprint(out.out, l.Formatter.Format(params))
			l.CheckFileSize(out)
			if err != nil {
				return
			}
		}
	}
}

func (l *Logger) SetLogPath(s string) {
	l.LogPath = s
	l.Outs = append(l.Outs, &LoggerWriter{
		Level: -1,
		out:   FileWriter(path.Join(s, "all.mslog")),
	})
	l.Outs = append(l.Outs, &LoggerWriter{
		Level: LevelDebug,
		out:   FileWriter(path.Join(s, "debug.mslog")),
	})
	l.Outs = append(l.Outs, &LoggerWriter{
		Level: LevelInfo,
		out:   FileWriter(path.Join(s, "info.mslog")),
	})
	l.Outs = append(l.Outs, &LoggerWriter{
		Level: LevelError,
		out:   FileWriter(path.Join(s, "error.mslog")),
	})
}

func (l *Logger) CheckFileSize(w *LoggerWriter) {
	//判断对应文件大小
	file := w.out.(*os.File)
	if file != nil {
		stat, err := file.Stat()
		if err != nil {
			return
		}
		size := stat.Size()
		if l.LogFilesize <= 0 {
			l.LogFilesize = 100 << 20
		}
		if size >= l.LogFilesize {
			_, name := path.Split(stat.Name())
			fileName := name[0:strings.Index(name, ".")]
			writer := FileWriter(path.Join(l.LogPath, msstrings.JoinStrings(fileName, ".", time.Now().Unix(), ".mslog")))
			w.out = writer
		}
	}
}

func Default() *Logger {
	logger := New()
	logger.Level = LevelDebug
	out := &LoggerWriter{
		Level: LevelDebug,
		out:   os.Stdout,
	}
	logger.Outs = append(logger.Outs, out)
	logger.Formatter = TextFormat
	return logger
}

func DefaultJson() *Logger {
	logger := Default()
	logger.Formatter = JsonFormat
	return logger
}

func FileWriter(filename string) io.Writer {
	file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return file
}
