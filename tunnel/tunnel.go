package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	logging "github.com/op/go-logging"
)

const (
	WRITE_TIMEOUT = 60000
)

var (
	logger = logging.MustGetLogger("msocks")
)

type Tunnel struct {
	net.Conn
	wlock     sync.Mutex
	closed    bool
	plock     sync.RWMutex
	next_id   uint16
	weaves    map[uint16]Fiber
	dft_fiber Fiber
}

func NewTunnel(conn net.Conn, next_id uint16) (t *Tunnel) {
	t = &Tunnel{
		Conn:    conn,
		closed:  false,
		next_id: next_id,
		weaves:  make(map[uint16]Fiber, 0),
	}
	return
}

func (t *Tunnel) PutIntoNextId(f Fiber) (id uint16, err error) {
	t.plock.Lock()

	startid := t.next_id
	for _, ok := t.weaves[t.next_id]; ok; _, ok = t.weaves[t.next_id] {
		t.next_id += 2
		if t.next_id == startid {
			err = errors.New("run out of stream id")
			logger.Error(err.Error())
			return
		}
	}
	id = t.next_id
	t.next_id += 2
	t.weaves[id] = f

	t.plock.Unlock()

	logger.Debugf("%s put into next id %d: %p.", t.String(), id, f)
	return
}

func (t *Tunnel) PutIntoId(id uint16, f Fiber) (err error) {
	logger.Debugf("%s put into id %d: %p.", t.String(), id, f)

	t.plock.Lock()

	_, ok := t.weaves[id]
	if ok {
		return ErrIdExist
	}
	t.weaves[id] = f

	f.plock.Unlock()
	return
}

func (t *Tunnel) SendFrame(f *Frame) (err error) {
	logger.Debugf("sent %s", f.Debug())

	b, err := f.Pack()
	if err != nil {
		return
	}

	t.wlock.Lock()
	t.Conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT * time.Millisecond))
	n, err := t.Conn.Write(b)
	t.wlock.Unlock()

	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	logger.Debugf("sess %s write %d bytes.", t.String(), len(b))
	return
}

func (t *Tunnel) CloseFiber(streamid uint16) (err error) {
	t.plock.Lock()
	_, ok := t.weaves[streamid]
	if !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(t.weaves, streamid)
	t.plock.Unlock()

	logger.Infof("%s remove port %d.", t.String(), streamid)
	return
}

func (t *Tunnel) Close() (err error) {
	defer t.Conn.Close()

	t.plock.RLock()
	defer t.plock.RUnlock()

	logger.Warningf("close all connects (%d) for session: %s.",
		len(t.weaves), t.String())

	for i, v := range t.weaves {
		go func() {
			err := v.Close(i)
			if err != nil {
				logger.Error(err.Error())
				return
			}
		}()
	}
	t.closed = true
	return
}

func (t *Tunnel) Loop() {
	defer t.Close()

	for {
		f, err := ReadFrame(t.Conn)
		switch err {
		default:
			logger.Error(err.Error())
			return
		case io.EOF:
			logger.Infof("%s read EOF", t.String())
			return
		case nil:
		}

		logger.Debugf("recv %s", f.Debug())

		t.plock.RLock()
		fiber, ok := t.weaves[f.FrameHeader.Streamid]
		t.plock.RUnlock()
		if !ok || fiber == nil {
			fiber = t.dft_fiber
		}

		err = fiber.SendFrame(f)
		if err != nil {
			logger.Errorf("send %s => %s(%d) failed, err: %s.",
				f.Debug(), s.String(), f.GetStreamid(), err.Error())
			return
		}
		// Don't do this now
		return ErrStreamNotExist
	}
}
