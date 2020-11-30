# gnmid (gNMI Target)

`gnmid` is a gNMI (gRPC Network Management Interface) target implementation to support the following gNMI RPC services.

- Capabilities RPC
- Subscribe RPC
- Get RPC
- Set RPC

## Overview of the sample implementation

```text
# Overview of the sample implementation.

+-----------+             +--------------------+              +-------------+
| subsystem | <--[YDB]--> | gnmid (gNMI Target)| <--[gRPC]--> | gNMI client |
+-----------+             +--------------------+              +-------------+

YDB: uss://gnmi channel

```

In this sample implementation, `gnmid` receives and updates NIC (Network Interface Card) statistics and syslog messages from a subsystem process. The subsystem collects the NIC statistics and then transfer it to `gnmid` via an YDB communication channel. The subsystem also runs as a syslog server to collect the syslog messages from the system. You have to configure a remote syslog server in your system if you want to enable the syslog messages over gNMI service.

## Syslog configuration

```bash
# rsyslog configuration to send the syslog messages to the subsystem
cat '*.*     @@localhost:11514` >> /etc/rsyslog.conf
```

## Install

You have to install [YAML DataBlock](https://github.com/neoul/libydb) for the gNMI target implementation.

```bash
# git clone https://github.com/neoul/libydb.git
#cd libydb
go get github.com/neoul/libydb
cd src/github.com/neoul/libydb
autoreconf -i -f
./configure
make
sudo make install
sudo ldconfig
```

## Run

```bash
./gnmid.demo.sh
```
