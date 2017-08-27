package dns

import (
	"sync"
	"testing"

	"github.com/shell909090/goproxy/netutil"
	"github.com/shell909090/goproxy/tunnel"
)

func TestTcpTunnel(t *testing.T) {
	var wg sync.WaitGroup
	tunnel.SetLogging()

	RegisterService()

	wg.Add(1)
	go func() {
		err := tunnel.RunMockServer(&wg)
		if err != nil {
			t.Error(err)
		}
		return
	}()
	wg.Wait()

	dc := tunnel.NewDialerCreator(
		netutil.DefaultTcpDialer, "tcp4", "127.0.0.1:14755", "", "")
	tun, err := dc.Create()
	if err != nil {
		t.Error(err)
		return
	}
	go func() {
		tun.Loop()
		logger.Warning("client loop quit")
	}()

	client := NewTcpClient(tun)
	_, err = client.LookupIP("www.google.com")
	if err != nil {
		t.Error(err)
		return
	}
}
