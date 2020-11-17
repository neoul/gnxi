#!/bin/bash

killall subsystem -q

cd subsystem
go build
./subsystem &
SUBSYS=$!

cd ..
go build
./gnmid \
  --v 2 \
  --cheat-code admin \
  --bind-address :10161 \
  --sync-path /interfaces/interface/state/counters \
  --server-key ../pki/server.key \
  --server-cert ../pki/server.crt \
  --ca-cert ../pki/ca.crt \
  --alsologtostderr \
  --stderrthreshold 0 \
  --cheat-code admin
