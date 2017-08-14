package tunnel

import (
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"
)

type Fabric struct {
	net.Conn
	wlock     sync.Mutex
	closed    bool
	plock     sync.RWMutex
	next_id   uint16
	weaves    map[uint16]Fiber
	dft_fiber Fiber
}

func NewFabric(conn net.Conn, next_id uint16) (fab *Fabric) {
	fab = &Fabric{
		Conn:    conn,
		closed:  false,
		next_id: next_id,
		weaves:  make(map[uint16]Fiber, 0),
	}
	return
}

func (fab *Fabric) String() string {
	return fmt.Sprintf(
		"%s->%s",
		fab.Conn.LocalAddr().String(),
		fab.Conn.RemoteAddr().String())
}

func (fab *Fabric) GetSize() int {
	fab.plock.Lock()
	defer fab.plock.Unlock()
	return len(fab.weaves)
}

type ConnSlice []*Conn

func (cs ConnSlice) Len() int           { return len(cs) }
func (cs ConnSlice) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs ConnSlice) Less(i, j int) bool { return cs[i].streamid < cs[j].streamid }

func (fab *Fabric) GetConnections() (conns ConnSlice) {
	conns = func() (conns []*Conn) {
		fab.plock.RLock()
		defer fab.plock.RUnlock()

		for _, f := range fab.weaves {
			if c, ok := f.(*Conn); ok {
				conns = append(conns, c)
			}
		}
		return
	}()
	sort.Sort(conns)
	return
}

func (fab *Fabric) PutIntoNextId(f Fiber) (id uint16, err error) {
	fab.plock.Lock()
	defer fab.plock.Unlock()

	startid := fab.next_id
	for _, ok := fab.weaves[fab.next_id]; ok; _, ok = fab.weaves[fab.next_id] {
		fab.next_id += 2
		if fab.next_id == startid {
			err = ErrStreamOutOfID
			logger.Error(err.Error())
			return
		}
	}
	id = fab.next_id
	fab.next_id += 2
	fab.weaves[id] = f

	logger.Debugf("%s put %p into %s.", fab.String(), f, id)
	return
}

func (fab *Fabric) PutIntoId(id uint16, f Fiber) (err error) {
	fab.plock.Lock()
	defer fab.plock.Unlock()

	_, ok := fab.weaves[id]
	if ok {
		return ErrIdExist
	}
	fab.weaves[id] = f

	logger.Debugf("%s put %p into %d.", fab.String(), f, id)
	return
}

func (fab *Fabric) SendFrame(f *Frame) (err error) {
	logger.Debugf("sent %s", f.Debug())

	b := f.Pack()

	fab.wlock.Lock()
	fab.Conn.SetWriteDeadline(
		time.Now().Add(WRITE_TIMEOUT * time.Millisecond))
	n, err := fab.Conn.Write(b)
	fab.wlock.Unlock()

	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	logger.Debugf("%s wrote len(%d).", fab.String(), len(b))
	return
}

func (fab *Fabric) CloseFiber(streamid uint16) (err error) {
	fab.plock.Lock()
	defer fab.plock.Unlock()
	if _, ok := fab.weaves[streamid]; !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(fab.weaves, streamid)

	logger.Infof("%s remove port %d.", fab.String(), streamid)
	return
}

func (fab *Fabric) Close() (err error) {
	defer fab.Conn.Close()

	fab.plock.RLock()
	defer fab.plock.RUnlock()
	if fab.closed {
		return
	}
	fab.closed = true

	logger.Warningf(
		"%s close all connects (%d).", fab.String(), len(fab.weaves))
	for i, f := range fab.weaves {
		go func(streamid uint16, fiber Fiber) {
			// conn.CloseFiber may call tunnel.CloseFiber,
			// which will try to lock tunnel.plock.
			// use goroutine to provent daedlock.
			err := fiber.CloseFiber(streamid)
			if err != nil {
				logger.Error(err.Error())
				return
			}
		}(i, f)
	}
	return
}

func (fab *Fabric) Loop() {
	defer fab.Close()

	for {
		f, err := ReadFrame(fab.Conn, nil)
		switch err {
		default:
			logger.Error(err.Error())
			return
		case io.EOF:
			logger.Infof("%s read EOF", fab.String())
			return
		case nil:
		}

		logger.Debugf("recv %s", f.Debug())

		fab.plock.RLock()
		fiber, ok := fab.weaves[f.Header.Streamid]
		fab.plock.RUnlock()
		if !ok || fiber == nil {
			fiber = fab.dft_fiber
		}

		err = fiber.SendFrame(f)
		if err != nil {
			logger.Errorf("send %s => %s(%d) failed, err: %s.",
				f.Debug(), fab.String(),
				f.Header.Streamid, err.Error())
			return
		}
	}
	return
}
