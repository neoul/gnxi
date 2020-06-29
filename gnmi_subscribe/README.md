# gNMI Get

A simple shell binary that performs a GET against a gNMI Target.

## Install

```bash
go get github.com/neoul/gnxi/gnmi_get
go install github.com/neoul/gnxi/gnmi_get
```

## Run

```bash
gnmi_get \
  -xpath "/system/openflow/agent/config/datapath-id" \
  -xpath "/system/openflow/controllers/controller[name=main]/connections/connection[aux-id=0]/state/address" \
  -target_addr localhost:10161 \
  -target_name hostname.com \
  -key client.key \
  -cert client.crt \
  -ca ca.crt \
  -username foo \
  -password bar \
  -alsologtostderr
```

```bash
go run gnmi_get.go \
  -xpath "/interfaces/interface[name=eth0]" \
  -xpath "/interfaces/interface[name=eth1]" \
  -target_addr 192.168.0.77:10161 \
  -target_name 192.168.0.77 \
  -key ../pki/client.key \
  -cert ../pki/client.crt \
  -ca ../pki/ca.crt \
  -username foo \
  -password bar \
  -alsologtostderr
```
