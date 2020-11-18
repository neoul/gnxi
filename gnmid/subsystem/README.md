# Subsystem

This is a sample process to push NIC statistics and syslog messages to `gnmid` via YDB (Yaml DataBlock) interface.

- The sample subsystem collects the statistics of the system's NICs monitored by `ifconfig` via YDB interface.
- It manufactures and pushs the collected statistics to `gnmid` via YDB communication channel.
- It receives the syslog messages and pushs the syslog messages to `gnmid`.
