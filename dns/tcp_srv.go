package dns

import (
	"net"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/tunnel"
)

type TcpServer struct {
	Exchanger
}

func (server *TcpServer) Handle(fabconn net.Conn) (err error) {
	conn, ok := fabconn.(*tunnel.Conn)
	if !ok {
		panic("proxy with no fab conn.")
	}

	err = conn.Accept()
	if err != nil {
		logger.Error(err.Error())
		return
	}

	defer conn.Close()
	ip := getRemoteIP(conn)

	var quiz *dns.Msg
	var resp *dns.Msg
	for {
		quiz, err = readMsg(conn)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		logger.Infof("dns query %s", quiz.Question[0].Name)

		appendEdns0Subnet(quiz, ip)

		// FIXME: look after timeout.
		resp, err = server.Exchanger.Exchange(quiz)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		err = writeMsg(conn, resp)
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}
	return
}

func getRemoteIP(conn net.Conn) (ip net.IP) {
	addr := conn.RemoteAddr()
	switch taddr := addr.(type) {
	case *net.TCPAddr:
		ip = taddr.IP
	case *net.UDPAddr:
		ip = taddr.IP
	case *tunnel.Addr:
		paddr, ok := taddr.Addr.(*net.TCPAddr)
		if !ok {
			panic("dns tcp server over a connection not tcp.")
		}
		ip = paddr.IP
	}
	return
}

func appendEdns0Subnet(m *dns.Msg, addr net.IP) {
	newOpt := true
	var o *dns.OPT
	for _, v := range m.Extra {
		if v.Header().Rrtype == dns.TypeOPT {
			o = v.(*dns.OPT)
			newOpt = false
			break
		}
	}
	if o == nil {
		o = new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
	}
	e := new(dns.EDNS0_SUBNET)
	e.Code = dns.EDNS0SUBNET
	e.SourceScope = 0
	e.Address = addr
	if e.Address.To4() == nil {
		e.Family = 2 // IP6
		e.SourceNetmask = net.IPv6len * 8
	} else {
		e.Family = 1 // IP4
		e.SourceNetmask = net.IPv4len * 8
	}
	o.Option = append(o.Option, e)
	if newOpt {
		m.Extra = append(m.Extra, o)
	}
}

func init() {
	httpsdns, err := NewHttpsDns(nil)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	server := &TcpServer{
		Exchanger: httpsdns,
	}
	tunnel.RegisterNetwork("dns", server)
}
