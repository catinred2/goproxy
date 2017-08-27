package tunnel

import (
	"fmt"
	"io"
	"net"
	"time"
)

type PasswordAuthenticator interface {
	AuthPass(string, string) bool
}

func AuthConn(auth PasswordAuthenticator, conn net.Conn) (err error) {
	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		logger.Errorf("auth timeout %s.", conn.RemoteAddr())
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
		logger.Errorf("user %s auth failed with password: %s.",
			auth.Username, auth.Password)
		err = WriteFrame(
			stream, MSG_RESULT, fauth.Header.Streamid, ERR_AUTH)
		if err != nil {
			return
		}
		err = fmt.Errorf("user %s auth failed, password:%s.",
			auth.Username, auth.Password)
		return
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

var ProtocolHandlers map[string]Handler

func init() {
	p := new(TcpProxy)
	ProtocolHandlers = map[string]Handler{
		"tcp":  p,
		"tcp4": p,
		"tcp6": p,
	}
}

func RegisterNetwork(network string, handler Handler) (ok bool) {
	_, ok = ProtocolHandlers[network]
	if ok {
		return false
	}
	ProtocolHandlers[network] = handler
	return true
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
	var c *Conn
	handler, ok := ProtocolHandlers[syn.Network]
	if !ok {
		logger.Errorf("unknown network: %s.", syn.Network)
		err = SendFrame(
			s.Fabric, MSG_RESULT, streamid, ERR_UNKNOWN_PROTOCOL)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		return
	}

	c, err = s.accept(streamid, syn)
	if err != nil {
		return
	}
	go handler.Handle(c)
	return
}

func (s *TunnelServer) accept(streamid uint16, syn *Syn) (c *Conn, err error) {
	c = NewConn(s.Fabric)
	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_RECV)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	c.streamid = streamid
	c.Network = syn.Network
	c.Address = syn.Address

	err = s.Fabric.PutIntoId(streamid, c)
	if err != nil {
		logger.Error(err.Error())
		err = SendFrame(
			s.Fabric, MSG_RESULT, streamid, ERR_IDEXIST)
		if err != nil {
			logger.Error(err.Error())
			return
		}
	}
	return
}

// never called as default fiber.
func (s *TunnelServer) CloseFiber(streamid uint16) (err error) {
	panic("server's CloseFiber should never been called.")
	return
}
