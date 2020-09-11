#!/bin/bash

killall sub.system -q

cd sub.system
go build
./sub.system &
SUBSYS=$!

cd ..
go build
./gnmi_target --config=data/config.yaml
