package sutils

import (
	"log"
	"net"
)

func TcpServer(addr string, handler func (net.Conn) (error)) (err error) {
	var conn net.Conn
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil { return }
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil { return }
	for {
		conn, err = listener.Accept()
		if err != nil { return }
		go func () {
			e := handler(conn)
			if e != nil { log.Println(e.Error()) }
		} ()
	}
	return
}