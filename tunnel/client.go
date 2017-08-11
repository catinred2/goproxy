package tunnel

import (
	"fmt"
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

func (dc *DialerCreator) Create() (sess *Session, err error) {
	logger.Noticef("msocks try to connect %s.", dc.serveraddr)

	conn, err := dc.Dialer.Dial("tcp4", dc.serveraddr)
	if err != nil {
		return
	}

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		logger.Noticef(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})
	defer func() {
		ti.Stop()
	}()

	if dc.username != "" || dc.password != "" {
		logger.Noticef("auth with username: %s, password: %s.",
			dc.username, dc.password)
	}

	fb := NewFrameAuth(0, dc.username, dc.password)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return
	}

	f, err := ReadFrame(conn)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameResult)
	if !ok {
		return nil, ErrUnexpectedPkg
	}

	if ft.Errno != ERR_NONE {
		conn.Close()
		return nil, fmt.Errorf("create connection failed with code: %d.", ft.Errno)
	}

	logger.Notice("auth passed.")

	sess = NewSession(conn, 0)
	return
}

type Client struct {
	Tunnel
}

func NewClient(conn *Conn) (client *Client) {
	client = &Client{
		Tunnel: NewTunnel(conn, 0),
	}
	client.dft_fiber = client
	return
}

func (client *Client) Dial(network, address string) (c *Conn, err error) {
	c = NewConn(s.Tunnel)
	streamid, err := client.Tunnel.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	logger.Infof("try dial %s => %s:%s.",
		s.Conn.RemoteAddr().String(), network, address)

	err = c.Connect(network, address)
	if err != nil {
		logger.Error(err.Error())
	}
	return
}

func (client *Client) SendFrame(f *Frame) (err error) {
	switch f.FrameHeader.Type {
	case MSG_SYN:
		err = s.onSyn(f)
	default:
		logger.Error(ErrUnexpectedPkg.Error())
		return
	}
}

// never called as default fiber.
func (client *Client) CloseFiber(streamid uint16) (err error) {
	return
}
