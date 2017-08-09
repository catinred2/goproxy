package msocks

import (
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

func (s *Session) Dial(network, address string) (c *Conn, err error) {
	c = NewConn(0, s, network, address)
	streamid, err := s.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	logger.Infof("try dial %s => %s.", s.Conn.RemoteAddr().String(), address)
	err = c.SendSynAndWait()
	if err != nil {
		return
	}

	return c, nil
}

func (s *Session) on_syn(ft *FrameSyn) (err error) {
	// lock streamid temporary, with status sync recved
	c := NewConn(ft.Streamid, s, ft.Network, ft.Address)
	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_RECV)
	if err != nil {
		return
	}

	err = s.PutIntoId(ft.Streamid, c)
	if err != nil {
		logger.Error(err.Error())

		fb := NewFrameResult(ft.Streamid, ERR_IDEXIST)
		err = s.SendFrame(fb)
		if err != nil {
			logger.Error(err.Error())
		}
		return
	}

	// it may toke long time to connect with target address
	// so we use goroutine to return back loop
	go func() {
		var err error
		var conn net.Conn
		logger.Debugf("try to connect %s => %s:%s.", c.String(), ft.Network, ft.Address)

		if dialer, ok := sutils.DefaultTcpDialer.(sutils.TimeoutDialer); ok {
			conn, err = dialer.DialTimeout(
				ft.Network, ft.Address, DIAL_TIMEOUT*time.Second)
		} else {
			conn, err = sutils.DefaultTcpDialer.Dial(ft.Network, ft.Address)
		}

		if err != nil {
			logger.Error(err.Error())
			fb := NewFrameResult(ft.Streamid, ERR_CONNFAILED)
			err = s.SendFrame(fb)
			if err != nil {
				logger.Error(err.Error())
			}
			c.Final()
			return
		}

		fb := NewFrameResult(ft.Streamid, ERR_NONE)
		err = s.SendFrame(fb)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		err = c.CheckAndSetStatus(ST_SYN_RECV, ST_EST)
		if err != nil {
			return
		}

		go sutils.CopyLink(conn, c)
		logger.Noticef("connected %s => %s:%s.",
			c.String(), ft.Network, ft.Address)
		return
	}()
	return
}
