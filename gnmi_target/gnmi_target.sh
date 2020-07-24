#!/bin/bash

go run gnmi_target.go \
  -v 2 \
  -bind_address :10161 \
  -key ../pki/server.key \
  -cert ../pki/server.crt \
  -ca ../pki/ca.crt \
  -alsologtostderr \
  -stderrthreshold 0 \
  -yaml-config ../gnmi/model/data/sample.yaml \
  -disable-update-bundling
