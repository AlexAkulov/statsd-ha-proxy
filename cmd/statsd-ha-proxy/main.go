package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AlexAkulov/statsd-ha-proxy/server"
	"github.com/AlexAkulov/statsd-ha-proxy/upstreams"
	"github.com/go-kit/kit/metrics/graphite"
	"github.com/op/go-logging"
	"github.com/spf13/pflag"
)

var (
	version   = "unknown"
	goVersion = "unknown"
	buildDate = "unknown"

	log *logging.Logger
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
		fmt.Println("version: ", version)
		fmt.Println("Goland version: ", goVersion)
		fmt.Println("Build Date: ", buildDate)
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

	var cache = make(chan []byte, config.CacheSize)

	// Selfstate metrics
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	} else {
		hostname = strings.Split(hostname, ".")[0]
	}

	var selfStateTicker *time.Ticker

	selfState := graphite.New(fmt.Sprintf("%s.statsite_proxy.%s.", config.Stats.GraphitePrefix, hostname), nil)
	if config.Stats.Enabled {
		selfStateTicker = time.NewTicker(60 * time.Second)
		cacheMaxSize := selfState.NewGauge("cache.max_size")
		cacheUsed := selfState.NewGauge("cache.used")

		go func(c <-chan time.Time) {
			for range c {
				var err error
				addr, err := net.ResolveTCPAddr("tcp", config.Stats.GraphiteURI)
				if err != nil {
					log.Errorf("Stats err: %v", err)
					continue
				}
				conn, err := net.DialTCP("tcp", nil, addr)
				if err != nil {
					log.Errorf("Stats err: %v", err)
					continue
				}
				cacheMaxSize.Set(float64(config.CacheSize))
				cacheUsed.Set(float64(len(cache)))
				log.Debugf("Cache used %d, max %d", len(cache), config.CacheSize)
				if _, err := selfState.WriteTo(conn); err != nil {
					log.Error("Stats during", "WriteTo", "err", err)
				}
				conn.Close()
			}
		}(selfStateTicker.C)
	}

	// Start Backends
	statsiteBackends := upstreams.Upstream{
		Log:                      log,
		Stats:                    selfState,
		Channel:                  cache,
		BackendsList:             config.Backends,
		BackendReconnectInterval: time.Millisecond * time.Duration(config.ReconnectInterval),
		BackendTimeout:           time.Millisecond * time.Duration(config.Timeout),
		SwitchLatency:            time.Millisecond * time.Duration(config.SwitchLatency),
	}

	statsiteBackends.Start()

	statsiteProxyServer := server.Server{
		Log:           log,
		Stats:         selfState,
		Channel:       cache,
		ConfigListen:  config.Listen,
		ConfigServers: config.Backends,
	}

	if err := statsiteProxyServer.Start(); err != nil {
		statsiteBackends.Stop()
		log.Fatal(err)
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

	if selfStateTicker != nil {
		selfStateTicker.Stop()
	}

}
