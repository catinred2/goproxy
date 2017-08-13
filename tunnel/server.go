package tunnel

import (
	"io"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type PasswordAuthenticator interface {
	AuthPass(string, string) bool
}

func AuthConn(auth PasswordAuthenticator, conn net.Conn) (err error) {
	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		logger.Noticef(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})

	err = onAuth(auth, conn)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	ti.Stop()
	return
}

func onAuth(author PasswordAuthenticator, stream io.ReadWriteCloser) (err error) {
	var auth Auth
	fauth, err := ReadFrame(stream, &auth)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	if fauth.Header.Type != MSG_AUTH {
		return ErrUnexpectedPkg
	}

	if !author.AuthPass(auth.Username, auth.Password) {
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

	err = WriteFrame(
		stream, MSG_RESULT, fauth.Header.Streamid, ERR_NONE)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	logger.Info("auth passed.")
	return
}

type Handler interface {
	Handle(net.Conn) error
}

type Server struct {
	Handler
}

func (server *Server) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		go func() {
			defer conn.Close()
			err = server.Handle(conn)
			if err != nil {
				logger.Error(err.Error())
			}
		}()
	}
	return
}

type TunnelServer struct {
	*Fabric
}

func NewTunnelServer(conn net.Conn) (s *TunnelServer) {
	s = &TunnelServer{
		Fabric: NewFabric(conn, 1),
	}
	s.Fabric.dft_fiber = s
	return
}

func (s *TunnelServer) SendFrame(f *Frame) (err error) {
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

func (s *TunnelServer) onSyn(streamid uint16, syn *Syn) (err error) {
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

func (s *TunnelServer) tcp_proxy(streamid uint16, syn *Syn) (err error) {
	c := NewConn(s.Fabric)
	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_RECV)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	c.streamid = streamid

	err = s.Fabric.PutIntoId(streamid, c)
	if err != nil {
		logger.Error(err.Error())

		err = SendFrame(
			s.Fabric, MSG_RESULT, streamid, ERR_IDEXIST)
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
				s.Fabric, MSG_RESULT, streamid, ERR_CONNFAILED)
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
			s.Fabric, MSG_RESULT, streamid, ERR_NONE)
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
func (s *TunnelServer) CloseFiber(streamid uint16) (err error) {
	panic("server's CloseFiber should never been called.")
	return
}
