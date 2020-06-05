#!/bin/bash

go run gnmid.go \
  -v 2 \
  -bind_address :10161 \
  -key ../pki/server.key \
  -cert ../pki/server.crt \
  -ca ../pki/ca.crt \
  -alsologtostderr
