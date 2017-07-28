package msocks

import (
	"io"
	"net"
	"time"
)

type Listener struct {
	net.Listener
	auth map[string]string
}

func NewListener(raw net.Listener, auth *map[string]string) (listener net.Listener) {
	listener = &Listener{
		Listener: raw,
		auth:     *auth,
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
	f, err := ReadFrame(stream)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		return ErrUnexpectedPkg
	}

	if ft.Username != "" || ft.Password != "" {
		logger.Noticef("auth with username: %s, password: %s.",
			ft.Username, ft.Password)
	}

	if l.auth != nil {
		password1, ok := l.auth[ft.Username]
		if !ok || (ft.Password != password1) {
			fb := NewFrameResult(ft.Streamid, ERR_AUTH)
			buf, err := fb.Packed()
			_, err = stream.Write(buf.Bytes())
			if err != nil {
				return err
			}
			return ErrAuthFailed
		}
	}

	fb := NewFrameResult(ft.Streamid, ERR_NONE)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = stream.Write(buf.Bytes())
	if err != nil {
		return
	}

	logger.Info("auth passed.")
	return
}
