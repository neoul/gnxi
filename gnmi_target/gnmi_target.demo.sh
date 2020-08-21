#!/bin/bash

killall sub.system -q

cd sub.system
go build
./sub.system &
SUBSYS=$!

cd ..
go build
./gnmi_target \
  -v 2 \
  -bind_address :10161 \
  -key ../pki/server.key \
  -cert ../pki/server.crt \
  -ca ../pki/ca.crt \
  -alsologtostderr \
  -stderrthreshold 0