package main

import (
	stdlog "log"
	"os"
	"sync"
	"time"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/sutils"
	"github.com/shell909090/goproxy/tunnel"
)

func SetLogging() {
	logBackend := logging.NewLogBackend(os.Stderr, "",
		stdlog.Ltime|stdlog.Lmicroseconds|stdlog.Lshortfile)
	logging.SetBackend(logBackend)
	logging.SetFormatter(
		logging.MustStringFormatter("%{module}[%{level}]: %{message}"))
	lv, _ := logging.LogLevel("INFO")
	logging.SetLevel(lv, "")
	return
}

func main() {
	SetLogging()

	var wg sync.WaitGroup
	wg.Add(2)
	go sutils.EchoServer(&wg)
	go func() {
		err := tunnel.RunMockServer(&wg)
		if err != nil {
			t.Error(err)
		}
		return
	}()
	wg.Wait()

	time.Sleep(10 * time.Minute)
}
