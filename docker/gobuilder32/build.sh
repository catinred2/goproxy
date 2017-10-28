#!/bin/bash

wget -O docker/gobuilder32/go-386.tar.gz "https://storage.googleapis.com/golang/go1.9.2.linux-386.tar.gz"
sudo docker build -t gobuilder32 docker/gobuilder32/
rm -f docker/gobuilder32/go-386.tar.gz
