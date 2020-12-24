
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GoDoc](https://godoc.org/github.com/neoul/gnxi?status.svg)](https://godoc.org/github.com/neoul/gnxi)
[![Go Report Card](https://goreportcard.com/badge/github.com/neoul/gnxi)](https://goreportcard.com/report/github.com/neoul/gnxi)
[![Build Status](https://travis-ci.org/google/gnxi.svg?branch=master)](https://travis-ci.org/google/gnxi)
[![codecov.io](https://codecov.io/github/google/gnxi/coverage.svg?branch=master)](https://codecov.io/github/google/gnxi?branch=master)

# gNxI Tools

*   gNMI - gRPC Network Management Interface
*   gNOI - gRPC Network Operations Interface

A collection of tools for Network Management that use the gNMI and gNOI protocols.

### Summary

_Note_: These tools are intended for testing and as reference implementation of the protocol.

*  [gNMI Capabilities](./gnmi_capabilities)
*  [gNMI Set](./gnmi_set)
*  [gNMI Get](./gnmi_get)
*  [gNMI Target](./gnmi_target)
*  [gNOI Cert](./gnoi_cert)
*  [gNOI Target](./gnoi_target)

### Documentation

*  See [gNMI Protocol documentation](https://github.com/openconfig/reference/tree/master/rpc/gnmi).
*  See [gNOI Protocol documentation](https://github.com/openconfig/gnoi).
*  See [Openconfig documentation](http://www.openconfig.net/).

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See Docker for instructions on how to test against network equipment.

### Prerequisites

Install __go__ in your system https://golang.org/doc/install. Requires golang1.7+.

### Clone

Clone the project to your __go__ source folder:
```
mkdir -p $GOPATH/src/github.com/google/
cd $GOPATH/src/github.com/google/
git clone https://github.com/neoul/gnxi.git
```

### Running

To run the binaries:

```
cd $GOPATH/src/github.com/neoul/gnxi/gnmi_get
go run ./gnmi_get.go
```

## Docker

[FAUCET](https://github.com/faucetsdn/gnmi) currently includes a [Dockerfile](https://github.com/faucetsdn/gnmi/blob/master/Dockerfile) to setup the environment that facilitates testing these tools against network equipment.

## Disclaimer

*  This is not an official Google product.
*  See [how to contribute](CONTRIBUTING.md).


## download all dependencies

```bash
go get -u -v -f all
# or
go get -u ./...
```


```bash
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key capabilities
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key sub --path "openconfig-interfaces:interfaces/interface"
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key sub --path "/interfaces/interface[name=lo]"
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key sub --path "/interfaces/interface[name=lo]" --path "/messages" --stream-mode ON_CHANGE
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key get --path "/interfaces/interface[name=lo]"
gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key set --delete "/interfaces/interface[name=lo]"

gnmic -a 192.168.0.77:10161 -u neoul -p admin --tls-ca pki/ca.crt --tls-cert pki/client.crt --tls-key pki/client.key sub --path "openconfig-interfaces:interfaces/interface/state/counters" --sample-interval 10s --heartbeat-interval 20s --stream-mode sample --suppress-redundant --log
```

## To Do

* Path Aliases processing
  * Add and delete path aliases
  * Client-defined Aliases
  * Target-defined Aliases
* Target and Origin in gNMI Path


## Not supported

- 3.5.2.1 Bundling of Telemetry Updates: Spec error described in https://github.com/openconfig/reference/issues/59
- 