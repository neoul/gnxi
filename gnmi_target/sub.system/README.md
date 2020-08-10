# Subsystem

This is a sample process to push gNMI data to `gnmi_target` via YDB (Yaml DataBlock) interface.

- The sample subsystem collects the statistics of the system's NICs monitored by `ifconfig` via YDB interface.
- It manufactures and pushs the collected stats to `gnmi_target` via YDB communication channel.

## Install

To push the gNMI data via YDB, [libydb](https://github.com/neoul/libydb) must be installed.

```bash
go get github.com/neoul/libydb
cd src/github.com/neoul/libydb
autoreconf -i -f
./configure
make
sudo make install
```

## Run

```bash
go run subsystem.go
```
