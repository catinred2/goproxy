package dns

import (
	"io"
	"net"
	"sync"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/netutil"
)

type TcpClient struct {
	Resolver
	lock   sync.RWMutex
	conn   net.Conn
	dialer netutil.Dialer
}

func NewTcpClient(dialer netutil.Dialer) (client *TcpClient) {
	client = &TcpClient{dialer: dialer}
	client.Resolver = &WrapExchanger{Exchanger: client}
	return
}

func (client *TcpClient) makeConn(create bool) (err error) {
	client.lock.Lock()
	defer client.lock.Unlock()

	if create && client.conn != nil {
		return
	}

	conn := client.conn
	err = conn.Close()
	if err != nil {
		return
	}
	client.conn = nil

	conn, err = client.dialer.Dial("dns", "")
	if err != nil {
		return
	}
	client.conn = conn
	return
}

func (client *TcpClient) Exchange(quiz *dns.Msg) (resp *dns.Msg, err error) {
	logger.Debugf("query %s", quiz.Question[0].Name)
	client.makeConn(true)

	for i := 0; i < 3; i++ {
		client.lock.RLock()
		resp, err = client.exchangeOnce(quiz)
		client.lock.RUnlock()
		switch err {
		case nil:
			return
		default:
			logger.Error(err.Error())
			continue
		case io.EOF:
			err = client.createConn(false)
			if err != nil {
				logger.Error(err.Error())
				return
			}
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
