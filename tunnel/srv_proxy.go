package tunnel

import (
	"net"
	"time"

	"github.com/shell909090/goproxy/netutil"
)

func DialMaybeTimeout(network, address string) (conn net.Conn, err error) {
	if dialer, ok := netutil.DefaultTcpDialer.(netutil.TimeoutDialer); ok {
		conn, err = dialer.DialTimeout(
			network, address, DIAL_TIMEOUT*time.Second)
	} else {
		conn, err = netutil.DefaultTcpDialer.Dial(network, address)
	}
	return
}

func tcp_proxy(c *Conn) {
	var err error
	var conn net.Conn

	logger.Debugf("%s try to connect %s:%s.",
		c.String(), c.Network, c.Address)

	conn, err = DialMaybeTimeout(c.Network, c.Address)
	if err != nil {
		logger.Error(err.Error())
		c.Deny()
		return
	}

	err = c.Accept()
	if err != nil {
		return
	}

	go netutil.CopyLink(conn, c)
	logger.Noticef("%s connected to %s:%s.",
		c.String(), c.Network, c.Address)
	return
}
