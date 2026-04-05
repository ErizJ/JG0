package config

import (
	"flag"
	"github.com/BurntSushi/toml"
	"github.com/ErizJ/JG0/mslog"
	"os"
)

var Conf = &MsConfig{
	Logger: mslog.Default(),
	Log:    make(map[string]any),
}

type MsConfig struct {
	Log    map[string]any
	Logger *mslog.Logger
}

func init() {
	LoadToml()
}

func LoadToml() {
	confFile := flag.String("conf", "config/app.toml", "app config file")
	flag.Parse()
	if _, err := os.Stat(*confFile); err != nil {
		Conf.Logger.Info("config/app.toml file not exist")
		return
	}
	_, err := toml.DecodeFile(*confFile, Conf)
	if err != nil {
		Conf.Logger.Info("config/app.toml decode fail check format")
		return
	}
}
