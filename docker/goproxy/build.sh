#!/bin/bash

cp bin/goproxy docker/goproxy/
strip -s docker/goproxy/goproxy
cp debian/routes.list.gz docker/goproxy/
sudo docker build -t goproxy docker/goproxy/
rm -f docker/goproxy/routes.list.gz
rm -f docker/goproxy/goproxy
