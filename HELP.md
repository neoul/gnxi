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

## gNMI client

### aristanetworks gNMI client

https://github.com/aristanetworks/goarista/tree/master/cmd/gnmi

```bash
gnmi -addr 192.168.0.77:10161 -tls -keyfile ../pki/client.key -cafile ../pki/ca.crt -certfile ../pki/client.crt -compression "" capabilities
gnmi -addr 192.168.0.77:10161 -tls -keyfile ../pki/client.key -cafile ../pki/ca.crt -certfile ../pki/client.crt -compression "" get '/interfaces/interface[name=eth0]'
gnmi -addr 192.168.0.77:10161 -tls -keyfile ../pki/client.key -cafile ../pki/ca.crt -certfile ../pki/client.crt -compression "" subscribe '/interfaces/interface[name=eth0]'
```

### OpenConf gNMI client

```bash
gnmi_cli -address 192.168.0.77:10161 -target 192.168.0.77 -ca_crt pki/ca.crt -client_crt pki/client.crt -client_key pki/client.key -capabilities
```

### [gRPC Web UI Client (grpcui)](https://github.com/fullstorydev/grpcui)

```bash
grpcui -cacert github.com/neoul/gnxi/pki/ca.crt -cert github.com/neoul/gnxi/pki/client.crt -key github.com/neoul/gnxi/pki/client.key -proto github.com/openconfig/gnmi/proto/gnmi_ext/gnmi_ext.proto -proto github.com/openconfig/gnmi/proto/gnmi/gnmi.proto 192.168.0.77:10161
```
