#!/bin/bash

wget "https://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz"
sudo docker build -t gobuilder .
rm -f go1.9.2.linux-amd64.tar.gz
