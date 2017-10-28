#!/bin/bash

wget -O docker/gobuilder/go-amd64.tar.gz "https://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz"
sudo docker build -t gobuilder docker/gobuilder
rm -f docker/gobuilder/go-amd64.tar.gz
