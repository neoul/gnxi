# gNxI Tools

## gNMI Server

- Update test cases to `server_test.go` using custom model in `github.com/neoul/gnxi/gnmi`.
- Add **YDB Go Udate Interface** to `modeldata.go` to update gostruct using YDB.
  - YDB Update Interface `{Create, Relace, Delete}` should be defined.
  - Add the test cases to `server_test.go` for the YDB Update Interface verification.
- Update deprecated `Descriptor()` function in `getGNMIServiceVersion()`
- `CapabilityResponse/gNMI_version` should follow [OpenConfig Semantic Versioning](http://openconfig.net/docs/semver/)
