package tunnel

import "io"

func myip(c *Conn) {
	err := c.Accept()
	if err != nil {
		logger.Error(err.Error())
		return
	}

	yourip := c.fab.RemoteAddr().String()
	logger.Infof("your ip: %s.", yourip)

	dat := []byte(yourip)
	n, err := c.Write(dat)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	if n < len(dat) {
		logger.Error(io.ErrShortWrite.Error())
		return
	}

	c.Close()
	return
}
