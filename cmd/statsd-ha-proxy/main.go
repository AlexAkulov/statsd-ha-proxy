package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bytes"

	"github.com/AlexAkulov/statsd-ha-proxy/server"
	"github.com/AlexAkulov/statsd-ha-proxy/upstreams"
	"github.com/op/go-logging"
	"github.com/spf13/pflag"
)

var (
	version = "unknown"
	log     *logging.Logger
)

func main() {
	versionFlag := pflag.BoolP("version", "v", false, "Print version and exit")
	configPath := pflag.StringP("config", "c", "config.yml", "Path to config file")
	helpFlag := pflag.BoolP("help", "h", false, "Print this message and exit")
	printDefaultConfigFlag := pflag.Bool("print-default-config", false, "Print default config and exit")

	pflag.Parse()

	if *helpFlag {
		pflag.PrintDefaults()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("version: %s\n", version)
		os.Exit(0)
	}

	if *printDefaultConfigFlag {
		printDefaultConfig()
		os.Exit(0)
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	log, err = newLog(config.LogFile, config.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var cache = make(chan *bytes.Buffer, config.CacheSize)

	// Start Backends
	statsiteBackends := upstreams.Upstream{
		Log:                      log,
		Channel:                  cache,
		BackendsList:             config.Backends,
		BackendReconnectInterval: config.ReconnectInterval,
		BackendTimeout:           config.Timeout,
		SwitchLatency:            config.SwitchLatency,
	}

	statsiteBackends.Start()

	statsiteProxyServer := server.Server{
		Log:           log,
		Channel:       cache,
		ConfigListen:  config.Listen,
		ConfigServers: config.Backends,
	}

	if err := statsiteProxyServer.Start(); err != nil {
		log.Critical(err)
	}

	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	log.Info(<-signalChannel)

	if err := statsiteProxyServer.Stop(); err != nil {
		log.Error(err)
	}

	if err := statsiteBackends.Stop(); err != nil {
		log.Error(err)
	}

}
