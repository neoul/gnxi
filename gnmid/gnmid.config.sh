#!/bin/bash

killall subsystem -q

cd subsystem
go build
./subsystem &
SUBSYS=$!

cd ..
go build
./gnmid --config=data/config.yaml
