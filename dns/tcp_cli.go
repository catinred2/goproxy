package dns

import (
	"net"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/netutil"
)

type TcpClient struct {
	Resolver
	conn   net.Conn
	dialer netutil.Dialer
}

func NewTcpClient(dialer netutil.Dialer) (client *TcpClient) {
	client = &TcpClient{dialer: dialer}
	client.Resolver = &WrapExchanger{Exchanger: client}
	return
}

func (client *TcpClient) Exchange(quiz *dns.Msg) (resp *dns.Msg, err error) {
	logger.Debugf("query %s", quiz.Question[0].Name)
	if client.conn == nil {
		// JIT, no warm up. Make things easier.
		client.conn, err = client.dialer.Dial("dns", "")
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}

	for i := 0; i < 3; i++ {
		resp, err = client.exchangeOnce(quiz)
		if err == nil {
			return
		}

		client.conn.Close()
		client.conn, err = client.dialer.Dial("dns", "")
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}
	return
}

func (client *TcpClient) exchangeOnce(quiz *dns.Msg) (resp *dns.Msg, err error) {
	err = writeMsg(client.conn, quiz)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	// FIXME: look after timeout.

	resp, err = readMsg(client.conn)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}
