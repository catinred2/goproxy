package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type DialerCreator struct {
	sutils.Dialer
	network    string
	serveraddr string
	username   string
	password   string
}

func NewDialerCreator(raw sutils.Dialer, network, serveraddr, username, password string) (dc *DialerCreator) {
	return &DialerCreator{
		Dialer:     raw,
		network:    network,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
}

func (dc *DialerCreator) Create() (client *Client, err error) {
	logger.Noticef("msocks try to connect %s.", dc.serveraddr)

	conn, err := dc.Dialer.Dial(dc.network, dc.serveraddr)
	if err != nil {
		return
	}

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		logger.Noticef(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})
	defer ti.Stop()

	if dc.username != "" || dc.password != "" {
		logger.Noticef("auth with username: %s, password: %s.",
			dc.username, dc.password)
	}

	auth := Auth{
		Username: dc.username,
		Password: dc.password,
	}
	err = WriteFrame(conn, MSG_AUTH, 0, &auth)
	if err != nil {
		return
	}

	var errno Result
	frslt, err := ReadFrame(conn, &errno)
	if err != nil {
		return
	}

	if frslt.Header.Type != MSG_RESULT {
		return nil, ErrUnexpectedPkg
	}
	if errno != ERR_NONE {
		conn.Close()
		return nil, fmt.Errorf("create connection failed with code: %d.", errno)
	}

	logger.Notice("auth passed.")
	client = NewClient(conn)
	return
}

type Client struct {
	*Tunnel
}

func NewClient(conn net.Conn) (client *Client) {
	client = &Client{
		Tunnel: NewTunnel(conn, 0),
	}
	client.dft_fiber = client
	return
}

func (client *Client) Dial(network, address string) (c *Conn, err error) {
	c = NewConn(client.Tunnel)
	c.streamid, err = client.Tunnel.PutIntoNextId(c)
	if err != nil {
		return
	}

	logger.Debugf("%s try to dial %s:%s.", client.String(), network, address)

	err = c.Connect(network, address)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Infof("%s connected.", c.String())
	return
}

func (client *Client) SendFrame(f *Frame) (err error) {
	panic("client should never recv unmapped frame.")
	return
}

func (client *Client) CloseFiber(streamid uint16) (err error) {
	panic("client's CloseFiber should never been called.")
	return
}
