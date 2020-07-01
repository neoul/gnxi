# gNMI CLI copy from openconfig/gnmi

```bash

## Capabilities
go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -capabilities

## Get
go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -get -proto \
'path: <
  elem: <
    name: "interfaces"
  >
  elem: <
    name: "interface"
    key: <
      key: "name"
      value: "eth0"
    >
  >
>'

## SubscribeRequest
go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -alsologtostderr -stderrthreshold 0 -v 2 -query "/interfaces/interface[name=eth0]"

```
