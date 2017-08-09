package msocks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"
)

type Session struct {
	wlock sync.Mutex
	conn  net.Conn

	closed  bool
	plock   sync.RWMutex
	next_id uint16
	ports   map[uint16]FrameSender

	Readcnt  *SpeedCounter
	Writecnt *SpeedCounter
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:     conn,
		closed:   false,
		next_id:  1,
		ports:    make(map[uint16]FrameSender, 0),
		Readcnt:  NewSpeedCounter(),
		Writecnt: NewSpeedCounter(),
	}
	logger.Notice("session %s created.", s.String())
	return
}

func (s *Session) String() string {
	return fmt.Sprintf("%d", s.LocalPort())
}

func (s *Session) GetSize() int {
	s.plock.RLock()
	defer s.plock.RUnlock()
	return len(s.ports)
}

func (s *Session) GetPorts() (ports []*Conn) {
	s.plock.RLock()
	defer s.plock.RUnlock()

	for _, fs := range s.ports {
		if c, ok := fs.(*Conn); ok {
			ports = append(ports, c)
		}
	}
	return
}

type ConnSlice []*Conn

func (cs ConnSlice) Len() int           { return len(cs) }
func (cs ConnSlice) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs ConnSlice) Less(i, j int) bool { return cs[i].streamid < cs[j].streamid }

func (s *Session) GetSortedPorts() (ports ConnSlice) {
	ports = s.GetPorts()
	sort.Sort(ports)
	return
}

func (s *Session) PutIntoNextId(fs FrameSender) (id uint16, err error) {
	s.plock.Lock()

	startid := s.next_id
	for _, ok := s.ports[s.next_id]; ok; _, ok = s.ports[s.next_id] {
		s.next_id += 1
		if s.next_id == startid {
			err = errors.New("run out of stream id")
			logger.Error("%s", err)
			return
		}
	}
	id = s.next_id
	s.next_id += 2
	s.ports[id] = fs

	s.plock.Unlock()

	logger.Debug("%s put into next id %d: %p.", s.String(), id, fs)
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) (err error) {
	logger.Debug("%s put into id %d: %p.", s.String(), id, fs)

	s.plock.Lock()

	_, ok := s.ports[id]
	if ok {
		return ErrIdExist
	}
	s.ports[id] = fs

	s.plock.Unlock()
	return
}

func (s *Session) RemovePort(streamid uint16) (err error) {
	s.plock.Lock()
	_, ok := s.ports[streamid]
	if !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(s.ports, streamid)
	s.plock.Unlock()

	logger.Info("%s remove port %d.", s.String(), streamid)
	return
}

func (s *Session) Close() (err error) {
	defer s.conn.Close()

	err = s.Readcnt.Close()
	if err != nil {
		return
	}
	err = s.Writecnt.Close()
	if err != nil {
		return
	}

	s.plock.RLock()
	defer s.plock.RUnlock()

	logger.Warning("close all connects (%d) for session: %s.",
		len(s.ports), s.String())

	for _, v := range s.ports {
		go v.CloseFrame()
	}
	s.closed = true
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) LocalPort() int {
	addr, ok := s.LocalAddr().(*net.TCPAddr)
	if !ok {
		return -1
	}
	return addr.Port
}

func (s *Session) SendFrame(f Frame) (err error) {
	logger.Debug("sent %s", f.Debug())
	s.Writecnt.Add(uint32(f.GetSize() + 5))

	buf, err := f.Packed()
	if err != nil {
		return
	}
	b := buf.Bytes()

	s.wlock.Lock()
	s.conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT * time.Second))
	n, err := s.conn.Write(b)
	s.wlock.Unlock()
	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	logger.Debug("sess %s write %d bytes.", s.String(), len(b))
	return
}

func (s *Session) CloseFrame() error {
	return s.Close()
}

func (s *Session) Run() {
	defer s.Close()

	for {
		f, err := ReadFrame(s.conn)
		switch err {
		default:
			logger.Error("%s", err)
			return
		case io.EOF:
			logger.Info("%s read EOF", s.String())
			return
		case nil:
		}

		logger.Debug("recv %s", f.Debug())
		s.Readcnt.Add(uint32(f.GetSize() + 5))

		switch ft := f.(type) {
		default:
			logger.Error("%s", ErrUnexpectedPkg.Error())
			return
		case *FrameResult, *FrameData, *FrameWnd, *FrameFin, *FrameRst:
			err = s.sendFrameInChan(f)
			if err != nil {
				logger.Error("send %s => %s(%d) failed, err: %s.",
					f.Debug(), s.String(), f.GetStreamid(), err.Error())
				return
			}
		case *FrameSyn:
			err = s.on_syn(ft)
			if err != nil {
				logger.Error("syn failed: %s", err.Error())
				return
			}
		case *FrameDns:
			err = s.on_dns(ft)
			if err != nil {
				logger.Error("dns failed: %s", err.Error())
				return
			}
		case *FramePing:
		case *FrameSpam:
		}
	}
}

// no drop, any error will reset main connection
func (s *Session) sendFrameInChan(f Frame) (err error) {
	streamid := f.GetStreamid()

	s.plock.RLock()
	c, ok := s.ports[streamid]
	s.plock.RUnlock()
	if !ok || c == nil {
		return ErrStreamNotExist
	}

	err = c.SendFrame(f)
	if err != nil {
		return
	}
	return nil
}
