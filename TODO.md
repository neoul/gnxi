# gNxI Tools

## gNMI Server

- Add **YDB Go Update Interface** to `modeldata/gostruct/ydb.go` to update gostruct using YDB.
  - YDB Update Interface `{Create, Relace, Delete}` should be defined. **(done)**
  - Add the test cases to `server_test.go` for the YDB Update Interface verification.
- Update deprecated `Descriptor()` function in `getGNMIServiceVersion()`
- `CapabilityResponse/gNMI_version` should follow [OpenConfig Semantic Versioning](http://openconfig.net/docs/semver/)

## YDB Interface

- Need unified logging facilities for YDB C library and YDB Go Interface. (c logging ==> go logging)

## gRPC

- Check the procedure of the process termination.

## Question

- How to enable the logging of gnxi?
- Need to check multi-threading of the gNMI server for Subscribe.
- Need to get the session information from gNMI or gRPC channel.

## Complete Items

- `ModelData` used in `Capabilities` is updated by ygot generation code automatically.
- Updated test cases to `server_test.go` using sample model in `github.com/neoul/gnxi/gnmi`. (**Only Get**)

### YDB Go Interface

- Updated the print level and format of `DebugValueString()`.
- Add the conversion function of an enum value to an integer for gnmi data construction.
  - `Create()`, `Replace()` and `Delete()` functions for `YDB Update Interface`
