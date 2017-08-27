package tunnel

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/shell909090/goproxy/netutil"
)

type Addr struct {
	net.Addr
	streamid uint16
}

func (a *Addr) String() (s string) {
	return fmt.Sprintf("%s(%d)", a.Addr.String(), a.streamid)
}

func RecvWithTimeout(ch chan uint32, t time.Duration) (errno uint32) {
	var ok bool
	ch_timeout := time.After(t)
	select {
	case errno, ok = <-ch:
		if !ok {
			return ERR_CLOSED
		}
	case <-ch_timeout:
		return ERR_TIMEOUT
	}
	return
}

// use lock to protect: status, window.
// SendFrame are not included.
type Conn struct {
	fab       *Fabric
	lock      sync.Mutex
	status    uint8
	streamid  uint16
	ch_syn    chan uint32
	t_closing *time.Timer

	r_rest []byte
	rqueue *Queue
	window int32
	wev    *sync.Cond

	Network string
	Address string
}

func NewConn(fab *Fabric) (c *Conn) {
	c = &Conn{
		status: ST_UNKNOWN,
		fab:    fab,
		rqueue: NewQueue(),
		window: WINDOWSIZE,
	}
	c.wev = sync.NewCond(&c.lock)
	return
}

func (c *Conn) String() (s string) {
	return fmt.Sprintf("%s(%d)", c.fab.String(), c.streamid)
}

func (c *Conn) GetStreamId() uint16 {
	// used by manager
	return c.streamid
}

func (c *Conn) GetStatusString() (st string) {
	// used by manager
	c.lock.Lock()
	status := c.status
	c.lock.Unlock()
	st, ok := StatusText[status]
	if !ok {
		panic(fmt.Sprintf("status %d not exist", status))
	}
	return
}

func (c *Conn) GetTarget() (s string) {
	// used by manager
	return fmt.Sprintf("%s:%s", c.Network, c.Address)
}

func (c *Conn) Connect(network, address string) (err error) {
	c.Network = network
	c.Address = address

	c.ch_syn = make(chan uint32, 0)
	defer func() {
		c.ch_syn = nil
	}()

	err = c.CheckAndSetStatus(ST_UNKNOWN, ST_SYN_SENT)
	if err != nil {
		return
	}

	syn := Syn{
		Network: network,
		Address: address,
	}
	err = SendFrame(c.fab, MSG_SYN, c.streamid, &syn)
	if err != nil {
		logger.Error(err.Error())
		c.Final()
		return
	}

	errno := RecvWithTimeout(c.ch_syn, DIAL_TIMEOUT*time.Millisecond)

	if errno != ERR_NONE {
		errtxt, ok := ErrnoText[errno]
		if !ok {
			errtxt = "unknown"
		}
		logger.Errorf(
			"%s connect %s:%s failed for %s",
			c.String(), network, address, errtxt)
		c.Final()
		return
	}
	err = c.CheckAndSetStatus(ST_SYN_SENT, ST_EST)
	return
}

func (c *Conn) Accept() (err error) {
	err = c.CheckAndSetStatus(ST_SYN_RECV, ST_EST)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	err = SendFrame(
		c.fab, MSG_RESULT, c.streamid, ERR_NONE)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}

func (c *Conn) Deny() (err error) {
	defer c.Final()
	err = SendFrame(
		c.fab, MSG_RESULT, c.streamid, ERR_CONNFAILED)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}

func (c *Conn) CheckAndSetStatus(old uint8, new uint8) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.status != old {
		err = ErrState
		logger.Error(err.Error())
		return
	}
	c.status = new
	return
}

func (c *Conn) Read(data []byte) (n int, err error) {
	var v interface{}
	target := data[:]
	for len(target) > 0 {
		if c.r_rest == nil {
			// when data isn't empty, reader should return.
			// when it is empty, reader should be blocked in here.
			v, err = c.rqueue.Pop(n == 0)
			if err != nil {
				return
			}

			if v == nil {
				// when rqueue not blocked
				// it will return v=nil, err=nil
				break
			}
			c.r_rest = v.([]byte)
		}

		size := copy(target, c.r_rest)
		target = target[size:]
		n += size

		if len(c.r_rest) > size {
			c.r_rest = c.r_rest[size:]
		} else {
			// take all data in rest
			c.r_rest = nil
		}
	}

	logger.Debugf("%s readed %d bytes.", c.String(), n)

	c.lock.Lock()
	defer c.lock.Unlock()

	// send wnd renew after fin will cause unmapped frame.
	switch c.status {
	case ST_FIN_SENT, ST_UNKNOWN:
		return
	}

	err = SendFrame(c.fab, MSG_WND, c.streamid, uint32(n))
	if err != nil {
		logger.Error(err.Error())
		return
	}
	return
}

func (c *Conn) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		size := uint16(len(data))
		if size > uint16(netutil.BUFFERSIZE) {
			size = uint16(netutil.BUFFERSIZE)
			// random size
			// size = uint16(16*1024 + rand.Intn(16*1024))
		}

		err = c.writeSlice(data[:size])
		switch err {
		default:
			logger.Error(err.Error())
			return
		case io.EOF:
			logger.Infof("%s connection closed.")
			return
		case nil:
		}
		logger.Debugf("%s send chunk [%d:%d+%d].", c.String(), n, n, size)

		data = data[size:]
		n += int(size)
	}
	logger.Debugf("%s sent %d bytes.", c.String(), n)
	return
}

func (c *Conn) writeSlice(data []byte) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.status != ST_EST {
		return io.ErrClosedPipe
	}

	fdata := NewFrame(MSG_DATA, c.streamid)
	fdata.Data = data
	fdata.Header.Length = uint16(len(data))

	logger.Debugf("write data len: %d, window: %d", len(data), c.window)
	for c.window-int32(len(data)) < 0 {
		// just one goroutine could wait here.
		c.wev.Wait()
	}

	err = c.fab.SendFrame(fdata)
	if err != nil {
		return
	}

	c.window -= int32(len(data))
	return
}

func (c *Conn) Close() (err error) {
	return c.closeWrite()
}

func (c *Conn) Reset() {
	c.lock.Lock()
	c.status = ST_UNKNOWN
	c.lock.Unlock()
	c.Final()
	err := c.rqueue.Close()
	if err != nil {
		panic(err.Error())
	}
}

func (c *Conn) Final() {
	err := c.fab.CloseFiber(c.streamid)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	logger.Noticef("%s final.", c.String())
	return
}

func (c *Conn) closeWrite() (err error) {
	// When sbd trying to close a conn, there should always have a daedline which
	// the connection can surely been closed.
	c.lock.Lock()
	defer c.lock.Unlock()

	switch c.status {
	case ST_EST:
		c.status = ST_FIN_SENT
		c.t_closing = time.AfterFunc(CLOSE_TIMEOUT*time.Millisecond, c.Reset)
	case ST_FIN_RECV:
		c.status = ST_UNKNOWN
		c.t_closing.Stop()
		c.t_closing = nil
		c.Final()
	case ST_UNKNOWN:
		return
	default:
		return ErrState
	}

	logger.Debugf("%s write close.", c.String())

	err = SendFrame(c.fab, MSG_FIN, c.streamid, nil)
	if err != nil {
		logger.Info(err.Error())
		return
	}

	return
}

func (c *Conn) closeRead() (err error) {
	logger.Debugf("%s read close.", c.String())
	c.lock.Lock()
	defer c.lock.Unlock()
	switch c.status {
	case ST_EST:
		c.status = ST_FIN_RECV
		c.t_closing = time.AfterFunc(CLOSE_TIMEOUT*time.Millisecond, c.Reset)
	case ST_FIN_SENT:
		c.status = ST_UNKNOWN
		c.t_closing.Stop()
		c.t_closing = nil
		c.Final()
	case ST_UNKNOWN:
		return
	default:
		return ErrState
	}

	c.rqueue.Close()
	return
}

func (c *Conn) LocalAddr() net.Addr {
	return &Addr{
		c.fab.LocalAddr(),
		c.streamid,
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return &Addr{
		c.fab.RemoteAddr(),
		c.streamid,
	}
}

func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SendFrame(f *Frame) (err error) {
	switch f.Header.Type {
	default:
		err = ErrUnexpectedPkg
		logger.Error(err.Error())
		c.Reset()

	case MSG_RESULT:
		c.lock.Lock()
		if c.status != ST_SYN_SENT {
			c.lock.Unlock()
			err = ErrState
			logger.Error(err.Error())
			return
		}
		c.lock.Unlock()

		var errno uint32
		err = f.Unmarshal(&errno)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		select {
		case c.ch_syn <- errno:
		default:
		}

	case MSG_DATA:
		err = c.rqueue.Push(f.Data)
		switch err {
		default:
			return
		case io.ErrClosedPipe:
			// Drop data here
			err = nil
		case nil:
		}
		logger.Debugf("%s recved %d bytes.", c.String(), len(f.Data))

	case MSG_WND:
		var window Wnd
		err = f.Unmarshal(&window)
		if err != nil {
			return
		}

		c.lock.Lock()
		c.window += int32(window)
		c.wev.Signal()
		c.lock.Unlock()
		logger.Debugf("%s window + %d = %d.", c.String(), window, c.window)

	case MSG_FIN:
		logger.Debugf("%s read close.", c.String())
		c.closeRead()

	case MSG_RST:
		logger.Debugf("%s reset.", c.String())
		c.Reset()
	}
	return
}

func (c *Conn) CloseFiber(streamid uint16) (err error) {
	// Mostly Fabric closed.
	c.Reset()
	return
}
