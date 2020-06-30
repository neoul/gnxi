# gNMI CLI copy from openconfig/gnmi

```bash

## Capabilities
gnmi_cli -address 192.168.0.77:10161 -ca_crt pki/ca.crt -client_crt pki/client.crt -client_key pki/client.key -capabilities

## Get
gnmi_cli -address 192.168.0.77:10161 -ca_crt pki/ca.crt -client_crt pki/client.crt -client_key pki/client.key -get -proto \
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
gnmi_cli -address 192.168.0.77:10161 -ca_crt pki/ca.crt -client_crt pki/client.crt -client_key pki/client.key -alsologtostderr -stderrthreshold 0 -v 2 -query "/interfaces/interface"

```
