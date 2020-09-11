# gNMI Target

gNMI (gRPC Network Management Interface) target (server) implementation.
The gNMI target supports followings.

- Capabilities
- Subscribe
- Get
- Set (Under development)

## Install

gNMI target requires [YAML DataBlock (https://github.com/neoul/libydb)](https://github.com/neoul/libydb) for data update.

```bash
git clone https://github.com/neoul/libydb.git
cd libydb
autoreconf -i -f
./configure CFLAGS="-g -Wall"
make
make install
```

```bash
go get github.com/neoul/gnxi/gnmi_target
go install github.com/neoul/gnxi/gnmi_target
```

## Run

```bash
./gnmi_target.demo.sh
```
