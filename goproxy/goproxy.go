package main

import (
	"encoding/json"
	"flag"
	"fmt"
	stdlog "log"
	"os"

	logging "github.com/op/go-logging"

	"github.com/shell909090/goproxy/sutils"
)

var logger = logging.MustGetLogger("")

var (
	ConfigFile string
)

type Config struct {
	Mode   string
	Listen string

	Logfile    string
	Loglevel   string
	AdminIface string

	DnsAddrs []string
	DnsNet   string
}

func init() {
	flag.StringVar(&ConfigFile, "config", "/etc/goproxy/config.json", "config file")
	flag.Parse()
}

func LoadJson(configfile string, cfg interface{}) (err error) {
	file, err := os.Open(configfile)
	if err != nil {
		return
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	err = dec.Decode(&cfg)
	return
}

func LoadConfig() (cfg Config, err error) {
	err = LoadJson(ConfigFile, &cfg)
	if err != nil {
		return
	}
	return
}

func SetLogging(cfg Config) (err error) {
	var file *os.File
	file = os.Stdout

	if cfg.Logfile != "" {
		file, err = os.OpenFile(cfg.Logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			logger.Fatal(err)
		}
	}
	logBackend := logging.NewLogBackend(file, "",
		stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.Lshortfile)
	logging.SetBackend(logBackend)

	logging.SetFormatter(logging.MustStringFormatter("%{level}: %{message}"))

	lv, err := logging.LogLevel(cfg.Loglevel)
	if err != nil {
		panic(err.Error())
	}
	logging.SetLevel(lv, "")

	return
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = SetLogging(cfg)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	switch cfg.DnsNet {
	case "internal":
	case "https":
		sutils.DefaultLookuper, err = sutils.NewGoogleHttpsDns()
		if err != nil {
			return
		}
	case "udp", "tcp":
		if len(cfg.DnsAddrs) > 0 {
			sutils.DefaultLookuper = sutils.NewDnsLookuper(
				cfg.DnsAddrs, cfg.DnsNet)
		}
	}

	switch cfg.Mode {
	case "server":
		logger.Notice("server mode start.")
		err = run_server(&cfg)
	case "http":
		logger.Notice("http mode start.")
		err = run_httproxy(&cfg)
	default:
		logger.Info("unknown mode")
		return
	}
	if err != nil {
		logger.Error("%s", err)
	}
	logger.Info("server stopped")
}
