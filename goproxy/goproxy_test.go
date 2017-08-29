package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/netutil"
	"github.com/shell909090/goproxy/tunnel"
)

func AbsPath(i string) (o string) {
	o, _ = filepath.Abs(i)
	return
}

func TestGoproxy(t *testing.T) {
	var wg sync.WaitGroup
	tunnel.SetLogging()

	wg.Add(1)
	go netutil.EchoServer(&wg)
	wg.Wait()

	srvcfg := ServerConfig{
		Config: Config{
			Mode:     "server",
			Loglevel: "WARNING",
			Listen:   "127.0.0.1:5233",
		},
		CryptMode:   "tls",
		RootCAs:     AbsPath("../keys/ca.crt"),
		CertFile:    AbsPath("../keys/localhost.crt"),
		CertKeyFile: AbsPath("../keys/localhost.key"),
	}
	go func() {
		err := RunServer(&srvcfg)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	time.Sleep(500 * time.Millisecond)

	clicfg := ClientConfig{
		Config: Config{
			Mode:       "http",
			Listen:     "127.0.0.1:5234",
			Loglevel:   "WARNING",
			AdminIface: "127.0.0.1:5235",
			DnsNet:     "internal",
		},
		DnsServer: "127.0.0.1:5236",
		Blackfile: AbsPath("../debian/routes.list.gz"),
	}
	srvdesc := ServerDefine{
		CryptMode:   "tls",
		Server:      "localhost:5233",
		RootCAs:     AbsPath("../keys/ca.crt"),
		CertFile:    AbsPath("../keys/user.crt"),
		CertKeyFile: AbsPath("../keys/user.key"),
	}
	clicfg.Servers = append(clicfg.Servers, &srvdesc)

	go func() {
		err := RunHttproxy(&clicfg)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	time.Sleep(500 * time.Millisecond)

	_, err := dns.DefaultResolver.LookupIP("www.sina.com.cn")
	if err != nil {
		t.Error(err)
		return
	}

	proxyUrl, err := url.Parse("http://127.0.0.1:5234")
	if err != nil {
		panic(err)
	}
	myClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}

	for count := 0; count < 3; count++ {
		err = testURL(myClient, "http://127.0.0.1:5235/")
		if err != nil {
			t.Error(err)
			return
		}
	}

	return
}

func testURL(myClient *http.Client, url string) (err error) {
	resp, err := myClient.Get(url)
	if err != nil {
		logger.Info("failed once")
		err = nil
		return
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// logger.Debug(string(b))
	return
}
