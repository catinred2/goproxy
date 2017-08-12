package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

func AbsPath(i string) (o string) {
	o, _ = filepath.Abs(i)
	return
}

func TestGoproxy(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go sutils.EchoServer(&wg)
	wg.Wait()

	srvcfg := ServerConfig{
		Config: Config{
			Mode:   "server",
			Listen: "127.0.0.1:5233",
		},
		CryptMode:   "tls",
		RootCAs:     AbsPath("../keys/ca.crt"),
		CertFile:    AbsPath("../keys/server.crt"),
		CertKeyFile: AbsPath("../keys/server.key"),
	}
	go func() {
		err := RunServer(&srvcfg)
		if err != nil {
			t.Error(err)
			return
		}
	}()

	clicfg := ClientConfig{
		Config: Config{
			Mode:       "http",
			Listen:     "127.0.0.1:5234",
			AdminIface: "127.0.0.1:5235",
		},
	}
	srvdesc := ServerDefine{
		CryptMode:   "tls",
		Server:      "127.0.0.1:5233",
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

	proxyUrl, err := url.Parse("http://127.0.0.1:5234")
	if err != nil {
		panic(err)
	}
	myClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}

	time.Sleep(2 * time.Second)

	resp, err := myClient.Get("http://127.0.0.1:5235/")
	if err != nil {
		t.Error(err)
		return
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Print(b)
	return
}
