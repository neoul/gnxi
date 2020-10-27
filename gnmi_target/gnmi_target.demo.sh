#!/bin/bash

killall sub.system -q

cd sub.system
go build
./sub.system &
SUBSYS=$!

cd ..
go build
./gnmi_target \
  --v 2 \
  --cheat-code admin \
  --bind-address :10161 \
  --sync-required-path /interfaces/interface/state/counters \
  --server-key ../pki/server.key \
  --server-cert ../pki/server.crt \
  --ca-cert ../pki/ca.crt \
  --alsologtostderr \
  --stderrthreshold 0 \
  --cheat-code admin
