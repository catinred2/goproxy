package tunnel

import (
	"io"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type Listener struct {
	net.Listener
	auth *map[string]string
}

func NewListener(original net.Listener, auth *map[string]string) (listener net.Listener) {
	listener = &Listener{
		Listener: original,
		auth:     auth,
	}
	return
}

func (l *Listener) Accept() (conn net.Conn, err error) {
	conn, err = l.Listener.Accept()
	if err != nil {
		return
	}

	logger.Noticef("connection come from: %s => %s.",
		conn.RemoteAddr(), conn.LocalAddr())

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		logger.Noticef(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})

	err = l.onAuth(conn)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	ti.Stop()
	return
}

func (l *Listener) onAuth(stream io.ReadWriteCloser) (err error) {
	fauth, err := ReadFrame(stream)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if fauth.FrameHeader.Type != MSG_AUTH {
		return ErrUnexpectedPkg
	}

	var auth Auth
	err = fauth.Unmarshal(&auth)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if auth.Username != "" || auth.Password != "" {
		logger.Noticef("auth with username: %s, password: %s.",
			auth.Username, auth.Password)
	}

	if l.auth != nil {
		password1, ok := (*l.auth)[auth.Username]
		if !ok || (auth.Password != password1) {
			var errno Result = ERR_AUTH
			frslt := NewFrame(MSG_RESULT, fauth.FrameHeader.Streamid)
			err = frslt.Marshal(&errno)
			if err != nil {
				logger.Error(err.Error())
				return
			}

			err = frslt.WriteTo(stream)
			if err != nil {
				logger.Error(err.Error())
				return
			}

			return ErrAuthFailed
		}
	}

	var errno Result = ERR_NONE
	frslt := NewFrame(MSG_RESULT, fauth.FrameHeader.Streamid)
	err = frslt.Marshal(&errno)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	err = frslt.WriteTo(stream)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	logger.Info("auth passed.")
	return
}

type Server struct {
	*Tunnel
}

func NewServer(conn net.Conn) (s *Server) {
	s = &Server{
		Tunnel: NewTunnel(conn, 1),
	}
	s.dft_fiber = s
	return
}

func (s *Server) SendFrame(f *Frame) (err error) {
	switch f.FrameHeader.Type {
	case MSG_SYN:
		err = s.onSyn(f)
	default:
		logger.Infof(f.Debug())
		logger.Error(ErrUnexpectedPkg.Error())
	}
	return
}

func (s *Server) onSyn(f *Frame) (err error) {
	var syn Syn
	err = f.Unmarshal(&syn)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	c := NewConn(s.Tunnel)
	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_RECV)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	err = s.Tunnel.PutIntoId(f.FrameHeader.Streamid, c)
	if err != nil {
		logger.Error(err.Error())

		frslt := NewFrame(MSG_RESULT, f.FrameHeader.Streamid)
		err = frslt.Marshal(ERR_IDEXIST)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		err = s.Tunnel.SendFrame(frslt)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		return
	}

	go func() {
		var err error
		var conn net.Conn

		logger.Debugf("%s try to connect %s:%s.",
			c.String(), syn.Network, syn.Address)

		if dialer, ok := sutils.DefaultTcpDialer.(sutils.TimeoutDialer); ok {
			conn, err = dialer.DialTimeout(
				syn.Network, syn.Address, DIAL_TIMEOUT*time.Second)
		} else {
			conn, err = sutils.DefaultTcpDialer.Dial(syn.Network, syn.Address)
		}

		if err != nil {
			logger.Error(err.Error())
			defer c.Final()

			frslt := NewFrame(MSG_RESULT, f.FrameHeader.Streamid)
			err = frslt.Marshal(ERR_CONNFAILED)
			if err != nil {
				logger.Error(err.Error())
				return
			}

			err = s.Tunnel.SendFrame(frslt)
			if err != nil {
				logger.Error(err.Error())
			}
			return
		}

		err = c.CheckAndSetStatus(ST_SYN_RECV, ST_EST)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		frslt := NewFrame(MSG_RESULT, f.FrameHeader.Streamid)
		err = frslt.Marshal(ERR_NONE)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		err = s.Tunnel.SendFrame(frslt)
		if err != nil {
			logger.Error(err.Error())
		}

		go sutils.CopyLink(conn, c)
		logger.Noticef("%s connected to %s:%s.",
			c.String(), syn.Network, syn.Address)
		return
	}()

	return
}

// never called as default fiber.
func (s *Server) CloseFiber(streamid uint16) (err error) {
	return
}
