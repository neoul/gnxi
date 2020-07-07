# Help for gNMI developement

## gNMI ProtoBuf UML generation

```bash
# Download and install protodot and graphviz
# https://github.com/seamia/protodot

protodot -src ../../go/src/github.com/openconfig/gnmi/proto/gnmi/gnmi.proto -inc ../../go/src/google/protobuf -output gnmi
protodot -src ../../go/src/github.com/openconfig/gnmi/proto/gnmi/gnmi.proto -inc ../../go/src/google/protobuf -output gnmi.Subscribe -select .Subscribe
protodot -src ../../go/src/github.com/openconfig/gnmi/proto/gnmi/gnmi.proto -inc ../../go/src/google/protobuf -output gnmi.Set -select .Set
protodot -src ../../go/src/github.com/openconfig/gnmi/proto/gnmi/gnmi.proto -inc ../../go/src/google/protobuf -output gnmi.Get -select .Get
protodot -src ../../go/src/github.com/openconfig/gnmi/proto/gnmi/gnmi.proto -inc ../../go/src/google/protobuf -output gnmi.Capabilities -select .Capabilities

```

### OpenConf gNMI client

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

### [gRPC Web UI Client (grpcui)](https://github.com/fullstorydev/grpcui)

```bash
grpcui -cacert github.com/neoul/gnxi/pki/ca.crt -cert github.com/neoul/gnxi/pki/client.crt -key github.com/neoul/gnxi/pki/client.key -proto github.com/openconfig/gnmi/proto/gnmi_ext/gnmi_ext.proto -proto github.com/openconfig/gnmi/proto/gnmi/gnmi.proto 192.168.0.77:10161
```


```text
gnmipath -> schema path (yang.Entry) ->  full path (element name and key) for gnmipath ->

pathEntity struct {
    origin string
    target string
    prefix []string
    path []string (wildcard *, ...) 로 변환
}


yang:prefix는 삭제

map[string]interface{}
    interface{} = map[string]interface{}
        ... []*Subscription

elem ->
* ->
... ->
add(prefix, path, yang.Entry, *Subscription)
del(prefix, path, yang.Entry, *Subscription) 
search(path []string) []*Subscription


path.go
```