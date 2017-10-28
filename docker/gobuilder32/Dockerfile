FROM i386/debian
WORKDIR /srv/
COPY go1.9.2.linux-386.tar.gz /srv/
ENV http_proxy="http://192.168.33.81:8118" https_proxy="http://192.168.33.81:8118"
ENV PATH=$PATH:/srv/go/bin/:/srv/gocode/bin GOPATH=/srv/gocode:/srv/go
VOLUME /srv/gocode/bin/ /srv/gocode/src/ /srv/myapp/
RUN apt-get update \
    && apt-get install -y devscripts dh-systemd \
    && tar xf /srv/go1.9.2.linux-386.tar.gz -C /srv/ \
    && mkdir -p /srv/gocode/bin/ /srv/gocode/src/ /srv/myapp/ \
    && rm -f /srv/go1.9.2.linux-386.tar.gz \
    && apt-get clean
