package msocks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type Session struct {
	*sutils.ExchangerToLookuper
	net.Conn
	wlock sync.Mutex

	closed  bool
	plock   sync.RWMutex
	next_id uint16
	ports   map[uint16]FrameSender

	Readcnt  *SpeedCounter
	Writecnt *SpeedCounter
}

func NewSession(conn net.Conn, next_id uint16) (s *Session) {
	s = &Session{
		Conn:     conn,
		closed:   false,
		next_id:  next_id,
		ports:    make(map[uint16]FrameSender, 0),
		Readcnt:  NewSpeedCounter(),
		Writecnt: NewSpeedCounter(),
	}
	s.ExchangerToLookuper = &sutils.ExchangerToLookuper{
		Exchanger: s,
	}
	logger.Noticef("session %s created.", s.String())
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

func (s *Session) GetPortById(id uint16) (FrameSender, error) {
	s.plock.Lock()
	defer s.plock.Unlock()

	c, ok := s.ports[id]
	if !ok || c == nil {
		return nil, ErrStreamNotExist
	}
	return c, nil
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
			logger.Error(err.Error())
			return
		}
	}
	id = s.next_id
	s.next_id += 2
	s.ports[id] = fs

	s.plock.Unlock()

	logger.Debugf("%s put into next id %d: %p.", s.String(), id, fs)
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) (err error) {
	logger.Debugf("%s put into id %d: %p.", s.String(), id, fs)

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

	logger.Infof("%s remove port %d.", s.String(), streamid)
	return
}

func (s *Session) Close() (err error) {
	defer s.Conn.Close()

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

	logger.Warningf("close all connects (%d) for session: %s.",
		len(s.ports), s.String())

	for _, v := range s.ports {
		go v.CloseFrame()
	}
	s.closed = true
	return
}

func (s *Session) LocalPort() int {
	addr, ok := s.LocalAddr().(*net.TCPAddr)
	if !ok {
		return -1
	}
	return addr.Port
}

func (s *Session) SendFrame(f Frame) (err error) {
	logger.Debugf("sent %s", f.Debug())
	s.Writecnt.Add(uint32(f.GetSize() + 5))

	buf, err := f.Packed()
	if err != nil {
		return
	}
	b := buf.Bytes()

	s.wlock.Lock()
	s.Conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT * time.Millisecond))
	n, err := s.Conn.Write(b)
	s.wlock.Unlock()
	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	logger.Debugf("sess %s write %d bytes.", s.String(), len(b))
	return
}

func (s *Session) CloseFrame() error {
	return s.Close()
}

func (s *Session) Run() {
	defer s.Close()

	for {
		f, err := ReadFrame(s.Conn)
		switch err {
		default:
			logger.Error(err.Error())
			return
		case io.EOF:
			logger.Infof("%s read EOF", s.String())
			return
		case nil:
		}

		logger.Debugf("recv %s", f.Debug())
		s.Readcnt.Add(uint32(f.GetSize() + 5))

		switch ft := f.(type) {
		default:
			logger.Errorf("%s", ErrUnexpectedPkg.Error())
			return
		case *FrameResult, *FrameData, *FrameWnd, *FrameFin, *FrameRst:
			err = s.sendFrameInChan(f)
			if err != nil {
				logger.Errorf("send %s => %s(%d) failed, err: %s.",
					f.Debug(), s.String(), f.GetStreamid(), err.Error())
				return
			}
		case *FrameSyn:
			err = s.on_syn(ft)
			if err != nil {
				logger.Errorf("syn failed: %s", err.Error())
				return
			}
		case *FrameDns:
			err = s.on_dns(ft)
			if err != nil {
				logger.Errorf("dns failed: %s", err.Error())
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
