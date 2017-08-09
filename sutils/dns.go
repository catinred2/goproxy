package sutils

import (
	"net"

	"github.com/miekg/dns"
)

func DebugDNS(quiz, resp *dns.Msg) {
	straddr := ""
	for _, a := range resp.Answer {
		switch ta := a.(type) {
		case *dns.A:
			straddr += ta.A.String() + ","
		case *dns.AAAA:
			straddr += ta.AAAA.String() + ","
		}
	}
	logger.Infof("dns result for %s is %s.", quiz.Question[0].Name, straddr)
	return
}

type DnsLookuper struct {
	Lookuper
	Servers []string
	c       *dns.Client
}

func NewDnsLookuper(servers []string, dnsnet string) (d *DnsLookuper) {
	d = &DnsLookuper{
		Servers: servers,
		c:       &dns.Client{},
	}
	d.Lookuper = &ExchangerToLookuper{
		Exchanger: d,
	}
	d.c.Net = dnsnet
	return d
}

func (d *DnsLookuper) Exchange(m *dns.Msg) (r *dns.Msg, err error) {
	for _, srv := range d.Servers {
		r, _, err = d.c.Exchange(m, srv)
		if err != nil {
			continue
		}
		if len(r.Answer) > 0 {
			return
		}
	}
	return
}

type ExchangerToLookuper struct {
	Exchanger
}

func (e2l *ExchangerToLookuper) query(host string, t uint16, addrs_in []net.IP) (addrs []net.IP, err error) {
	addrs = addrs_in

	quiz := new(dns.Msg)
	quiz.SetQuestion(dns.Fqdn(host), t)
	quiz.RecursionDesired = true

	resp, err := e2l.Exchanger.Exchange(quiz)
	if err != nil {
		return
	}

	if DEBUGDNS {
		DebugDNS(quiz, resp)
	}

	for _, a := range resp.Answer {
		switch ta := a.(type) {
		case *dns.A:
			addrs = append(addrs, ta.A)
		case *dns.AAAA:
			addrs = append(addrs, ta.AAAA)
		}
	}
	return
}

func (e2l *ExchangerToLookuper) LookupIP(host string) (addrs []net.IP, err error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return []net.IP{ip}, nil
	}

	addrs, err = e2l.query(host, dns.TypeA, addrs)
	if err != nil {
		return
	}
	// CAUTION: disabled ipv6
	// addrs, err = e2l.query(host, dns.TypeAAAA, addrs)
	return
}
