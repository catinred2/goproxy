package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strings"

	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/msocks"
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

type PortMap struct {
	Net string
	Src string
	Dst string
}

type ClientConfig struct {
	Config
	Blackfile string

	MinSess int
	MaxConn int
	Servers []*ServerDefine

	HttpUser     string
	HttpPassword string

	Portmaps []PortMap
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
	var RootCAs *x509.CertPool
	var cert tls.Certificate

	if strings.ToLower(sd.CryptMode) == "tls" {
		cert, err = tls.LoadX509KeyPair(sd.CertFile, sd.CertKeyFile)
		if err != nil {
			return
		}

		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if sd.RootCAs != "" {
			RootCAs, err = loadCertPool(sd.RootCAs)
			if err != nil {
				return
			}
			config.RootCAs = RootCAs
		}

		dialer = &TlsDialer{config: config}
		return
	} else {
		cipher := sd.Cipher
		if cipher == "" {
			cipher = sd.Cipher
		}
		dialer, err = cryptconn.NewDialer(sutils.DefaultTcpDialer, cipher, sd.Key)
		if err != nil {
			return
		}
	}
	return
}

func run_httproxy(basecfg *Config) (err error) {
	cfg, err := LoadClientConfig(basecfg)
	if err != nil {
		return
	}

	var dialer sutils.Dialer
	sp := msocks.CreateSessionPool(cfg.MinSess, cfg.MaxConn)

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
		NewMsocksManager(sp).Register(mux)
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
		go CreatePortmap(pm, dialer)
	}

	return http.ListenAndServe(cfg.Listen, proxy.NewProxy(dialer, cfg.HttpUser, cfg.HttpPassword))
}
