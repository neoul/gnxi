# gnmid (gNMI Target)

`gnmid` is gNMI (gRPC Network Management Interface) target (server) implementation.
The `gnmid` supports gNMI RPC services as follows.

- Capabilities RPC
- Subscribe RPC
- Get RPC
- Set RPC

In this sample implementation, `gnmid` receives and updates NIC statistics and syslog messages from subsystem to provide the gNMI RPC service for openconfig-interfaces and openconfig-messages models.
To enable and receive the syslog messages from subsystem, you have to configure the syslog redirection to `localhost:11514`.

```bash
cat '*.*     @@localhost:11514` >> /etc/rsyslog.conf
```

```text

+-----------+             +--------------------+              +-------------+
| subsystem | <--[YDB]--> | gnmid (gNMI Target)| <--[gRPC]--> | gNMI client |
+-----------+             +--------------------+              +-------------+

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
