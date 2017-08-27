package main

import (
	"net/http"
	"strings"

	"github.com/shell909090/goproxy/connpool"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/netutil"
	"github.com/shell909090/goproxy/portmapper"
	"github.com/shell909090/goproxy/proxy"
	"github.com/shell909090/goproxy/tunnel"
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

	Portmaps  []portmapper.PortMap
	DnsServer string
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

func (sd *ServerDefine) MakeDialer() (dialer netutil.Dialer, err error) {
	if strings.ToLower(sd.CryptMode) == "tls" {
		dialer, err = NewTlsDialer(sd.CertFile, sd.CertKeyFile, sd.RootCAs)
	} else {
		cipher := sd.Cipher
		if cipher == "" {
			cipher = "aes"
		}
		dialer, err = cryptconn.NewDialer(netutil.DefaultTcpDialer, cipher, sd.Key)
	}
	return
}

func RunHttproxy(cfg *ClientConfig) (err error) {
	var dialer netutil.Dialer
	pool := connpool.NewDialer(cfg.MinSess, cfg.MaxConn)

	for _, srv := range cfg.Servers {
		dialer, err = srv.MakeDialer()
		if err != nil {
			return
		}
		creator := tunnel.NewDialerCreator(
			dialer, "tcp4", srv.Server, srv.Username, srv.Password)
		pool.AddDialerCreator(creator)
	}

	dialer = pool

	if cfg.DnsNet == "internal" {
		dns.DefaultResolver = dns.NewTcpClient(dialer)
	}

	if cfg.DnsServer != "" {
		go RunDnsServer(cfg.DnsServer)
	}

	if cfg.AdminIface != "" {
		mux := http.NewServeMux()
		pool.Register(mux)
		go httpserver(cfg.AdminIface, mux)
	}

	if cfg.Blackfile != "" {
		fdialer := ipfilter.NewFilteredDialer(dialer)
		err = fdialer.LoadFilter(netutil.DefaultTcpDialer, cfg.Blackfile)
		if err != nil {
			logger.Error("%s", err.Error())
			return
		}
		dialer = fdialer
	}

	// FIXME: port mapper?
	for _, pm := range cfg.Portmaps {
		go portmapper.CreatePortmap(pm, dialer)
	}

	p := proxy.NewProxy(dialer, cfg.HttpUser, cfg.HttpPassword)
	return http.ListenAndServe(cfg.Listen, p)
}
