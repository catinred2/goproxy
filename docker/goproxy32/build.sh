#!/bin/bash

cp bin/goproxy docker/goproxy32/
strip -s docker/goproxy32/goproxy
cp debian/routes.list.gz docker/goproxy32/
sudo docker build -t goproxy32 docker/goproxy32/
rm -f docker/goproxy32/routes.list.gz
rm -f docker/goproxy32/goproxy
