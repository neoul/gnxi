# gNMI Target

A simple shell binary that implements a gNMI Target with in-memory configuration and telemetry.

## Install

```bash
go get github.com/neoul/gnxi/gnmi_target
go install github.com/neoul/gnxi/gnmi_target
```

## Run

```bash
ydb -r pub -a uss://openconfig -f ../gnmi/modeldata/data/m6424.sample.yaml -d -v debug
```

```bash
gnmi_target \
  -bind_address :10161 \
  -config openconfig-openflow.json \
  -key server.key \
  -cert server.crt \
  -ca ca.crt \
  -username foo \
  -password bar \
  -alsologtostderr
```

```bash
go run gnmi_target.go \
  -bind_address :10161 \
  -config openconfig-openflow.json \
  -key ../pki/server.key \
  -cert ../pki/server.crt \
  -ca ../pki/ca.crt \
  -username foo \
  -password bar \
  -alsologtostderr
```
