### Makefile --- 

## Author: shell909090@gmail.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
LEVEL=NOTICE

all: download build

download:
	go get -u -d github.com/shell909090/goproxy/goproxy

build:
	mkdir -p bin
	go build -o bin/goproxy github.com/shell909090/goproxy/goproxy

clean:
	rm -rf bin pkg
	debclean

build-tar: build
	strip bin/goproxy
	tar cJf ../goproxy-`uname -m`.tar.xz bin/goproxy debian/config.json debian/routes.list.gz

build-deb: download
	dpkg-buildpackage
	mkdir debuild
	mv ../goproxy_* debuild

test:
	go test github.com/shell909090/goproxy/tunnel
	# go test github.com/shell909090/goproxy/dns
	go test github.com/shell909090/goproxy/ipfilter
	# go test github.com/shell909090/goproxy/goproxy

install: build
	install -d $(DESTDIR)/usr/bin/
	install -m 755 -s bin/goproxy $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 debian/routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 debian/config.json $(DESTDIR)/etc/goproxy/

### Makefile ends here
