# gNMI Capabilities

A simple shell binary that requests for Capabilities from a gNMI Target.

## Install

```bash
go get github.com/neoul/gnxi/gnmi_capabilities
go install github.com/neoul/gnxi/gnmi_capabilities
```

## Run

```bash
gnmi_capabilities \
  -target_addr localhost:10161 \
  -target_name hostname.com \
  -key client.key \
  -cert client.crt \
  -ca ca.crt \
  -alsologtostderr
```

```bash
go run gnmi_capabilities.go \
  -target_addr localhost:10161 \
  -target_name 192.168.0.77 \
  -key ../pki/client.key \
  -cert ../pki/client.crt \
  -ca ../pki/ca.crt \
  -alsologtostderr \
  -username root \
  -password admin
```

## To Be Check

- What is target_name?