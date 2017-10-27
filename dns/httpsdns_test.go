package dns

import (
	"testing"

	"github.com/shell909090/goproxy/tunnel"
)

func TestHttpsDns(t *testing.T) {
	tunnel.SetLogging()

	// query with subnet included here
	httpsdns, err := NewHttpsDns(nil)
	if err != nil {
		t.Error(err)
		return
	}
	// httpsdns.MyIP = "114.114.114.114"

	_, err = httpsdns.LookupIP("www.baidu.com")
	if err != nil {
		t.Error(err)
		return
	}
}
