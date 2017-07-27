package main

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strings"

	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/sutils"
)

type ServerConfig struct {
	Config
	CryptMode   string
	RootCAs     string
	CertFile    string
	CertKeyFile string
	ForceIPv4   bool
	Cipher      string
	Key         string
	Auth        map[string]string
}

func LoadServerConfig(basecfg *Config) (cfg *ServerConfig, err error) {
	err = LoadJson(ConfigFile, &cfg)
	if err != nil {
		return
	}
	cfg.Config = *basecfg
	if cfg.Cipher == "" {
		cfg.Cipher = "aes"
	}
	return
}

func listener_use_tls(raw net.Listener, cfg *ServerConfig) (wrapped net.Listener, err error) {
	var RootCAs *x509.CertPool

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.CertKeyFile)
	if err != nil {
		return
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if cfg.RootCAs != "" {
		RootCAs, err = loadCertPool(cfg.RootCAs)
		if err != nil {
			return
		}
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = RootCAs
	}

	wrapped = tls.NewListener(raw, config)
	return
}

func run_server(basecfg *Config) (err error) {
	cfg, err := LoadServerConfig(basecfg)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return
	}

	if strings.ToLower(cfg.CryptMode) == "tls" {
		listener, err = listener_use_tls(listener, cfg)
	} else {
		listener, err = cryptconn.NewListener(listener, cfg.Cipher, cfg.Key)
	}
	if err != nil {
		return
	}

	var dialer sutils.Dialer = sutils.DefaultTcpDialer
	if cfg.ForceIPv4 {
		logger.Info("force ipv4 dailer.")
		dialer = sutils.DefaultTcp4Dialer
	}

	svr, err := msocks.NewServer(cfg.Auth, dialer)
	if err != nil {
		return
	}

	if cfg.AdminIface != "" {
		mux := http.NewServeMux()
		NewMsocksManager(svr.SessionPool).Register(mux)
		go httpserver(cfg.AdminIface, mux)
	}

	return svr.Serve(listener)
}
