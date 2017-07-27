package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

var ErrLoadPEM = errors.New("certpool: append cert to pem failed")

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

type TlsDialer struct {
	config *tls.Config
}

func (td *TlsDialer) Dial(network, address string) (net.Conn, error) {
	return tls.Dial(network, address, td.config)
}

func (td *TlsDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	return tls.DialWithDialer(d, network, address, td.config)
}
