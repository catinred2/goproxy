package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

var ErrLoadPEM = errors.New("certpool: append cert to pem failed")

var CipherSuites []uint16 = []uint16{
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

var CurvePreferences []tls.CurveID = []tls.CurveID{
	tls.X25519,
}

func loadCertPool(caCerts string) (caCertPool *x509.CertPool, err error) {
	var pemCert []byte
	caCertPool = x509.NewCertPool()
	for _, certpath := range strings.Split(caCerts, "\n") {
		pemCert, err = ioutil.ReadFile(certpath)
		if err != nil {
			return
		}
		if !caCertPool.AppendCertsFromPEM(pemCert) {
			return nil, ErrLoadPEM
		}
	}
	return
}

func TlsListener(raw net.Listener, CertFile, CertKeyFile, RootCAs string) (wrapped net.Listener, err error) {
	cert, err := tls.LoadX509KeyPair(CertFile, CertKeyFile)
	if err != nil {
		return
	}

	config := &tls.Config{
		Certificates:     []tls.Certificate{cert},
		CipherSuites:     CipherSuites,
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: CurvePreferences,
	}

	if RootCAs != "" {
		config.ClientCAs, err = loadCertPool(RootCAs)
		if err != nil {
			return
		}
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	wrapped = tls.NewListener(raw, config)
	return
}

type TlsDialer struct {
	config *tls.Config
}

func NewTlsDialer(CertFile, CertKeyFile, RootCAs string) (dialer sutils.Dialer, err error) {
	cert, err := tls.LoadX509KeyPair(CertFile, CertKeyFile)
	if err != nil {
		return
	}

	config := &tls.Config{
		Certificates:     []tls.Certificate{cert},
		CipherSuites:     CipherSuites,
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: CurvePreferences,
	}

	if RootCAs != "" {
		config.RootCAs, err = loadCertPool(RootCAs)
		if err != nil {
			return
		}
	}

	dialer = &TlsDialer{config: config}
	return
}

func (td *TlsDialer) Dial(network, address string) (net.Conn, error) {
	return tls.Dial(network, address, td.config)
}

func (td *TlsDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	return tls.DialWithDialer(d, network, address, td.config)
}
