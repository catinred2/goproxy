package main

import (
	"github.com/miekg/dns"
	mydns "github.com/shell909090/goproxy/dns"
)

type DnsServer struct {
	mydns.Exchanger
}

func (dnssrv *DnsServer) ServeDNS(w dns.ResponseWriter, quiz *dns.Msg) {
	logger.Debugf("dns server query: %s", quiz.Question[0].Name)
	resp, err := dnssrv.Exchanger.Exchange(quiz)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	err = w.WriteMsg(resp)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}

func RunDnsServer(addr string) {
	handler := new(DnsServer)
	exhg, ok := mydns.DefaultResolver.(mydns.Exchanger)
	handler.Exchanger = exhg
	if !ok {
		panic("DefaultResolver not Exchanger?")
	}

	server := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: handler,
	}

	logger.Infof("dns server start.")
	go server.ListenAndServe()
}
