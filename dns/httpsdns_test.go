package dns

import "testing"

func TestHttpsDns(t *testing.T) {
	// query with subnet included here
	httpsdns, err := NewHttpsDns(nil)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = httpsdns.LookupIP("www.google.com")
	if err != nil {
		t.Error(err)
		return
	}
}
