package msocks

import (
	"net"
	"time"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/sutils"
)

func (s *Session) Exchange(quiz *dns.Msg) (resp *dns.Msg, err error) {
	cfs := CreateChanFrameSender(0)
	streamid, err := s.PutIntoNextId(&cfs)
	if err != nil {
		return
	}
	defer func() {
		err := s.RemovePort(streamid)
		if err != nil {
			logger.Error(err.Error())
		}
	}()

	b, err := quiz.Pack()
	if err != nil {
		return
	}
	fquiz := NewFrameDns(streamid, b)

	err = s.SendFrame(fquiz)
	if err != nil {
		return
	}

	ft, err := cfs.RecvWithTimeout(DNS_TIMEOUT * time.Second)
	if err != nil {
		return
	}

	fresp, ok := ft.(*FrameDns)
	if !ok {
		return nil, ErrDnsMsgIllegal
	}

	resp = &dns.Msg{}
	err = resp.Unpack(fresp.Data)
	if err != nil || !resp.Response || resp.Id != quiz.Id {
		return nil, ErrDnsMsgIllegal
	}
	return
}

func (s *Session) on_dns(ft *FrameDns) (err error) {
	req := new(dns.Msg)
	err = req.Unpack(ft.Data)
	if err != nil {
		return ErrDnsMsgIllegal
	}

	if req.Response {
		// ignore send fail, maybe just timeout.
		// should I log this ?
		return s.sendFrameInChan(ft)
	}

	logger.Infof("dns query for %s.", req.Question[0].Name)
	if ipaddr, ok := s.Conn.RemoteAddr().(*net.IPAddr); ok {
		appendEdns0Subnet(req, ipaddr.IP)
	}

	xchg, ok := sutils.DefaultLookuper.(sutils.Exchanger)
	if !ok {
		err = ErrDnsLookuper
		logger.Error(err.Error())
		return
	}

	res, err := xchg.Exchange(req)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}

	if sutils.DEBUGDNS {
		sutils.DebugDNS(req, res)
	}

	// send response back from streamid
	b, err := res.Pack()
	if err != nil {
		logger.Error(ErrDnsMsgIllegal.Error())
		return nil
	}

	fr := NewFrameDns(ft.GetStreamid(), b)
	err = s.SendFrame(fr)
	return
}

func appendEdns0Subnet(msg *dns.Msg, addr net.IP) {
	newOpt := true
	var o *dns.OPT
	for _, v := range msg.Extra {
		if v.Header().Rrtype == dns.TypeOPT {
			o = v.(*dns.OPT)
			newOpt = false
			break
		}
	}
	if o == nil {
		o = new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
	}
	e := &dns.EDNS0_SUBNET{
		Code:        dns.EDNS0SUBNET,
		SourceScope: 0,
		Address:     addr,
	}
	if addr.To4() == nil {
		e.Family = 2 // IP6
		e.SourceNetmask = net.IPv6len * 8
	} else {
		e.Family = 1 // IP4
		e.SourceNetmask = net.IPv4len * 8
	}
	o.Option = append(o.Option, e)
	if newOpt {
		msg.Extra = append(msg.Extra, o)
	}
}
