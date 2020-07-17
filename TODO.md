# TODO

## To Do list

## OpenConfig models to be defined

- **ModelData**: `openconfig-catalog-types.yang`, `openconfig-module-catalog.yang`
- **Subscription**: `openconfig-telemetry.yang`

## gNMI Server

- Add **YDB Go Update Interface** to `model/gostruct/ydb.go` to update gostruct using YDB.
  - YDB Update Interface `{Create, Relace, Delete}` should be defined. **(done)**
  - Add the test cases to `server_test.go` for the YDB Update Interface verification.
- Update deprecated `Descriptor()` function in `getGNMIServiceVersion()`
- `CapabilityResponse/gNMI_version` should follow [OpenConfig Semantic Versioning](http://openconfig.net/docs/semver/)
- `util/entity`: Self-signed certificate generation (need to check)

### Get RPC

#### GetRequest

- `type[CONFIG, STATE, OPERATIONAL]` (DataType) are **NOT** supported. (Just supports `type[ALL]`)
- `encoding[BYTES, PROTO, ASCII]` (Encoding) are **NOT** supported. (Just supports `encoding[JSON, JSON_IETF]`)
- `use_models` (ModelData) is not verified on the reception.

## [gRPC]

- Check the procedure of the process termination.
- feature check: gRPC server worker

## [Question]

- How to enable the logging of gnxi?
- Need to check multi-threading of the gNMI server for Subscribe.
- Need to get the session information from gNMI or gRPC channel.

---

## **Completed**

- `ModelData` used in `Capabilities` is updated by ygot generation code automatically.
- Updated test cases to `server_test.go` using sample model in `github.com/neoul/gnxi/gnmi`. (**Only Get**)
- Added YAML configuration file loading option on `gnmid` startup.

### YDB Go Interface

- YDB Go Interface supports the data construction from `YDB` to `ygot` completely.
  - `Create()`, `Replace()` and `Delete()`
- Unified the logging facilities for YDB C and Go Interface. (c logging ==> go logging)
