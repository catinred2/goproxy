package msocks

import "io"

type Server struct {
	userpass map[string]string
}

func NewServer(auth map[string]string) (ms *Server) {
	ms = &Server{
		userpass: auth,
	}

	return
}

func (ms *Server) OnAuth(stream io.ReadWriteCloser) (err error) {
	f, err := ReadFrame(stream)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		return ErrUnexpectedPkg
	}

	if ft.Username != "" || ft.Password != "" {
		logger.Notice("auth with username: %s, password: %s.", ft.Username, ft.Password)
	}
	if ms.userpass != nil {
		password1, ok := ms.userpass[ft.Username]
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
