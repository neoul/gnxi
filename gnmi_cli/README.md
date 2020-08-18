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
  elem: <
    name: "config"
  >
>'

go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -get -proto \
'prefix: <
  elem: <
    name: "interfaces"
  >
>
path: <
  elem: <
    name: "interface"
    key: <
      key: "name"
      value: "eth0"
    >
  >
>
path: <
  elem: <
    name: "interface"
    key: <
      key: "name"
      value: "eth1"
    >
  >
>'

## SubscribeRequest
go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -alsologtostderr -stderrthreshold 0 -v 2 -query "/interfaces/interface[name=wlp0s20f3]"

go run gnmi_cli.go -address 192.168.0.77:10161 -ca_crt ../pki/ca.crt -client_crt ../pki/client.crt -client_key ../pki/client.key -alsologtostderr -stderrthreshold 0 -v 2 -query "/interfaces/interface[name=eth0]/name" -query_type s
```

```bash
gnmic -a 192.168.0.77:10161 -u admin -p admin --tls-ca ../pki/ca.crt --tls-cert ../pki/client.crt --tls-key ../pki/client.key capabilities
gnmic -a 192.168.0.77:10161 -u admin -p admin --tls-ca ../pki/ca.crt --tls-cert ../pki/client.crt --tls-key ../pki/client.key sub --path "openconfig-interfaces:interfaces/interface"
gnmic -a 192.168.0.77:10161 -u admin -p admin --tls-ca ../pki/ca.crt --tls-cert ../pki/client.crt --tls-key ../pki/client.key sub --path "/interfaces/interface[name=wlp0s20f3]"
gnmic -a 192.168.0.77:10161 -u admin -p admin --tls-ca ../pki/ca.crt --tls-cert ../pki/client.crt --tls-key ../pki/client.key sub --path "/interfaces/interface[name=wlp0s20f3],/messages" --stream-mode ON_CHANGE
```
