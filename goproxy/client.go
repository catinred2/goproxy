package main

import (
	"net/http"
	"strings"

	"github.com/shell909090/goproxy/connpool"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/portmapper"
	"github.com/shell909090/goproxy/proxy"
	"github.com/shell909090/goproxy/sutils"
)

type ServerDefine struct {
	Server      string
	CryptMode   string
	RootCAs     string
	CertFile    string
	CertKeyFile string
	Cipher      string
	Key         string
	Username    string
	Password    string
}

type ClientConfig struct {
	Config
	Blackfile string

	MinSess int
	MaxConn int
	Servers []*ServerDefine

	HttpUser     string
	HttpPassword string

	Portmaps []portmapper.PortMap
}

func LoadClientConfig(basecfg *Config) (cfg *ClientConfig, err error) {
	err = LoadJson(ConfigFile, &cfg)
	if err != nil {
		return
	}
	cfg.Config = *basecfg
	if cfg.MaxConn == 0 {
		cfg.MaxConn = 16
	}
	return
}

func httpserver(addr string, handler http.Handler) {
	for {
		err := http.ListenAndServe(addr, handler)
		if err != nil {
			logger.Error("%s", err.Error())
			return
		}
	}
}

func (sd *ServerDefine) MakeDialer() (dialer sutils.Dialer, err error) {
	if strings.ToLower(sd.CryptMode) == "tls" {
		dialer, err = NewTlsDialer(sd.CertFile, sd.CertKeyFile, sd.RootCAs)
	} else {
		cipher := sd.Cipher
		if cipher == "" {
			cipher = "aes"
		}
		dialer, err = cryptconn.NewDialer(sutils.DefaultTcpDialer, cipher, sd.Key)
	}
	return
}

func run_httproxy(basecfg *Config) (err error) {
	cfg, err := LoadClientConfig(basecfg)
	if err != nil {
		return
	}

	var dialer sutils.Dialer
	sp := connpool.NewDialer(cfg.MinSess, cfg.MaxConn)

	for _, srv := range cfg.Servers {
		dialer, err = srv.MakeDialer()
		if err != nil {
			return
		}
		sp.AddSessionFactory(dialer, srv.Server, srv.Username, srv.Password)
	}

	dialer = sp

	if cfg.DnsNet == TypeInternal {
		sutils.DefaultLookuper = sp
	}

	if cfg.AdminIface != "" {
		mux := http.NewServeMux()
		sp.Register(mux)
		go httpserver(cfg.AdminIface, mux)
	}

	if cfg.Blackfile != "" {
		fdialer := ipfilter.NewFilteredDialer(dialer)
		err = fdialer.LoadFilter(sutils.DefaultTcpDialer, cfg.Blackfile)
		if err != nil {
			logger.Error("%s", err.Error())
			return
		}
		dialer = fdialer
	}

	for _, pm := range cfg.Portmaps {
		go portmapper.CreatePortmap(pm, dialer)
	}

	return http.ListenAndServe(cfg.Listen, proxy.NewProxy(dialer, cfg.HttpUser, cfg.HttpPassword))
}
