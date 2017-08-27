package dns

import (
	"errors"
	"net"

	"github.com/miekg/dns"
	logging "github.com/op/go-logging"
)

var (
	logger   = logging.MustGetLogger("dns")
	DEBUGDNS = true
)

var (
	ErrMessageTooLarge = errors.New("message body too large")
)

type Resolver interface {
	LookupIP(host string) (addrs []net.IP, err error)
}

// type NetResolver struct {
// }

// func (n *NetResolver) LookupIP(host string) (addrs []net.IP, err error) {
// 	return net.LookupIP(host)
// }

var DefaultResolver Resolver

type Exchanger interface {
	Exchange(*dns.Msg) (*dns.Msg, error)
}

type WrapExchanger struct {
	Exchanger
}

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

func (wrap *WrapExchanger) query(host string, t uint16, addrs *[]net.IP) (err error) {
	quiz := new(dns.Msg)
	quiz.SetQuestion(dns.Fqdn(host), t)
	quiz.RecursionDesired = true

	resp, err := wrap.Exchanger.Exchange(quiz)
	if err != nil {
		return
	}

	if DEBUGDNS {
		DebugDNS(quiz, resp)
	}

	for _, a := range resp.Answer {
		switch ta := a.(type) {
		case *dns.A:
			*addrs = append(*addrs, ta.A)
		case *dns.AAAA:
			*addrs = append(*addrs, ta.AAAA)
		}
	}
	return
}

func (wrap *WrapExchanger) LookupIP(host string) (addrs []net.IP, err error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return []net.IP{ip}, nil
	}

	err = wrap.query(host, dns.TypeA, &addrs)
	if err != nil {
		return
	}
	// CAUTION: disabled ipv6
	// err = wrap.query(host, dns.TypeAAAA, &addrs)
	return
}

type Dns struct {
	Resolver
	Servers []string
	client  *dns.Client
}

func NewDns(servers []string, dnsnet string) (d *Dns) {
	d = &Dns{
		Servers: servers,
		client: &dns.Client{
			Net: dnsnet,
		},
	}
	d.Resolver = &WrapExchanger{
		Exchanger: d,
	}
	return d
}

func (d *Dns) Exchange(m *dns.Msg) (r *dns.Msg, err error) {
	for _, srv := range d.Servers {
		r, _, err = d.client.Exchange(m, srv)
		if err != nil {
			continue
		}
		if len(r.Answer) > 0 {
			return
		}
	}
	return
}

func init() {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return
	}

	var addrs []string
	for _, srv := range conf.Servers {
		addrs = append(addrs, net.JoinHostPort(srv, conf.Port))
	}

	DefaultResolver = NewDns(addrs, "")
}
