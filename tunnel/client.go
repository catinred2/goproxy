package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type DialerCreator struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
}

func NewDialerCreator(raw sutils.Dialer, serveraddr, username, password string) (dc *DialerCreator) {
	return &DialerCreator{
		Dialer:     raw,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
}

func (dc *DialerCreator) Create() (client *Client, err error) {
	logger.Noticef("msocks try to connect %s.", dc.serveraddr)

	// FIXME: network?
	conn, err := dc.Dialer.Dial("tcp4", dc.serveraddr)
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
	fauth := NewFrame(MSG_AUTH, 0)
	err = fauth.Marshal(&auth)
	if err != nil {
		return
	}
	b := fauth.Pack()

	_, err = conn.Write(b)
	if err != nil {
		return
	}

	frslt, err := ReadFrame(conn)
	if err != nil {
		return
	}

	if frslt.FrameHeader.Type != MSG_RESULT {
		return nil, ErrUnexpectedPkg
	}

	var errno Result
	err = frslt.Unmarshal(&errno)
	if err != nil {
		return
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
	streamid, err := client.Tunnel.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	logger.Infof("try to make %s dial %s:%s.",
		client.Conn.RemoteAddr().String(), network, address)

	err = c.Connect(network, address)
	if err != nil {
		logger.Error(err.Error())
	}
	return
}

func (client *Client) SendFrame(f *Frame) (err error) {
	panic("why?")
	// switch f.FrameHeader.Type {
	// case MSG_SYN:
	// 	err = client.onSyn(f)
	// default:
	// 	logger.Error(ErrUnexpectedPkg.Error())
	// 	return
	// }
}

// never called as default fiber.
func (client *Client) CloseFiber(streamid uint16) (err error) {
	return
}
