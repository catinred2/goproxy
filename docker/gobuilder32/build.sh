#!/bin/bash

wget "https://storage.googleapis.com/golang/go1.9.2.linux-386.tar.gz"
sudo docker build -t gobuilder32 .
rm -f go1.9.2.linux-386.tar.gz
