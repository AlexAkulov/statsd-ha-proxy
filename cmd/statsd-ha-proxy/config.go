package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

type config struct {
	LogFile           string   `yaml:"log_file"`
	LogLevel          string   `yaml:"log_level"`
	Listen            string   `yaml:"listen"`
	Backends          []string `yaml:"servers"`
	Timeout           int64    `yaml:"timeout"`
	ReconnectInterval int64    `yaml:"reconnect_interval"`
	CacheSize         int64    `yaml:"cache_size"`
	SwitchLatency     int64    `yaml:"switch_upstream_latency"`
}

func printDefaultConfig() {
	c := getDefaultConfig()
	d, _ := yaml.Marshal(&c)
	fmt.Print(string(d))
}

func getDefaultConfig() config {
	return config{
		LogFile:  "stdout",
		LogLevel: "debug",
		Listen:   ":8125",
		Backends: []string{
			"statsite1:8125",
			"statsite2:8125",
		},
		Timeout:           1000,
		ReconnectInterval: 10000,
		CacheSize:         1000000,
		SwitchLatency:     10, //seconds
	}
}

func loadConfig(configPath string) (*config, error) {
	config := getDefaultConfig()
	configYAML, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("Can't open config file. %s", err)
	}
	err = yaml.Unmarshal([]byte(configYAML), &config)
	if err != nil {
		return nil, fmt.Errorf("Can't parse config file [%s] [%s]", configPath, err)
	}
	return &config, nil
}

func newLog(logFile, level string) (*logging.Logger, error) {
	logLevel, err := logging.LogLevel(level)
	if err != nil {
		logLevel = logging.DEBUG
	}
	var logBackend *logging.LogBackend
	if logFile == "stdout" || logFile == "" {
		logBackend = logging.NewLogBackend(os.Stdout, "", 0)
		logBackend.Color = true
	} else {
		logFileName := logFile
		logFile, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("Can't open log file %s: %s", logFileName, err.Error())
		}
		logBackend = logging.NewLogBackend(logFile, "", 0)
	}
	logger := logging.MustGetLogger("module")
	leveledLogBackend := logging.AddModuleLevel(logBackend)
	leveledLogBackend.SetLevel(logLevel, "module")
	logger.SetBackend(leveledLogBackend)
	return logger, nil
}
