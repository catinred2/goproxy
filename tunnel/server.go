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
	var auth Auth
	fauth, err := ReadFrame(stream, &auth)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if fauth.Header.Type != MSG_AUTH {
		return ErrUnexpectedPkg
	}

	if l.auth != nil {
		password1, ok := (*l.auth)[auth.Username]
		if !ok || (auth.Password != password1) {
			logger.Noticef("user %s auth failed with password: %s.",
				auth.Username, auth.Password)
			err = WriteFrame(
				stream, MSG_RESULT, fauth.Header.Streamid, ERR_AUTH)
			if err != nil {
				logger.Error(err.Error())
				return
			}
			return ErrAuthFailed
		}
	}

	err = WriteFrame(
		stream, MSG_RESULT, fauth.Header.Streamid, ERR_NONE)
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
	switch f.Header.Type {
	case MSG_SYN:
		var syn Syn
		err = f.Unmarshal(&syn)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		err = s.onSyn(f.Header.Streamid, &syn)
	default:
		err = ErrUnexpectedPkg
		logger.Infof(f.Debug())
		logger.Error(err.Error())
	}
	return
}

func (s *Server) onSyn(streamid uint16, syn *Syn) (err error) {
	switch syn.Network {
	default:
		err = ErrUnknownNetwork
		logger.Error(err.Error())
		return
	case "tcp", "tcp4", "tcp6":
		return s.tcp_proxy(streamid, syn)
	}
	return
}

func (s *Server) tcp_proxy(streamid uint16, syn *Syn) (err error) {
	c := NewConn(s.Tunnel)
	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_RECV)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	c.streamid = streamid

	err = s.Tunnel.PutIntoId(streamid, c)
	if err != nil {
		logger.Error(err.Error())

		err = SendFrame(
			s.Tunnel, MSG_RESULT, streamid, ERR_IDEXIST)
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

		conn, err = DialMaybeTimeout(syn.Network, syn.Address)
		if err != nil {
			logger.Error(err.Error())
			defer c.Final()

			err = SendFrame(
				s.Tunnel, MSG_RESULT, streamid, ERR_CONNFAILED)
			if err != nil {
				logger.Error(err.Error())
				return
			}
			return
		}

		err = c.CheckAndSetStatus(ST_SYN_RECV, ST_EST)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		err = SendFrame(
			s.Tunnel, MSG_RESULT, streamid, ERR_NONE)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		go sutils.CopyLink(conn, c)
		logger.Noticef("%s connected to %s:%s.",
			c.String(), syn.Network, syn.Address)
		return
	}()

	return
}

func DialMaybeTimeout(network, address string) (conn net.Conn, err error) {
	if dialer, ok := sutils.DefaultTcpDialer.(sutils.TimeoutDialer); ok {
		conn, err = dialer.DialTimeout(
			network, address, DIAL_TIMEOUT*time.Second)
	} else {
		conn, err = sutils.DefaultTcpDialer.Dial(network, address)
	}
	return
}

// never called as default fiber.
func (s *Server) CloseFiber(streamid uint16) (err error) {
	panic("server's CloseFiber should never been called.")
	return
}
