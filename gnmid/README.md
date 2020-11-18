# gnmid (gNMI Target)

`gnmid` is gNMI (gRPC Network Management Interface) target (server) implementation.
The `gnmid` supports gNMI RPC services as follows.

- Capabilities RPC
- Subscribe RPC
- Get RPC
- Set RPC

In this sample implementation, `gnmid` receives and updates NIC (Network Interface Card) statistics and syslog messages from subsystem to provide the gNMI RPC service for openconfig-interfaces and openconfig-messages models.
The subsystem collects the NIC statistics and then transfer it to `gnmid` via YDB communication channel. The subsystem also runs as a syslog server to collect the syslog messages from the system. So, you have to configure the syslog remote server (`localhost:11514`) in your syslog system.

```bash
# rsyslog configuration to send the syslog messages to the subsystem
cat '*.*     @@localhost:11514` >> /etc/rsyslog.conf
```

```text
# Overview of the sample implementation.

+-----------+             +--------------------+              +-------------+
| subsystem | <--[YDB]--> | gnmid (gNMI Target)| <--[gRPC]--> | gNMI client |
+-----------+             +--------------------+              +-------------+

YDB: uss://gnmi channel

```

## Install

This sample implementation requires `libydb` [YAML DataBlock (https://github.com/neoul/libydb)](https://github.com/neoul/libydb).

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
