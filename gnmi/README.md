# gNMI modeldata generation

1. Add YANG modules to `/yang`.
2. Update `gostruct/gen-xx.go` for generate Go Structure.
    - Generate code for the special target. e.g. `go generate gen-m6424.go`
3. Update modeldata `modeldata/modeldata.go` for capabilities exchange?.
    - Check the revision and openconfig version.
4. Copy ydb go interface (Updater interface).
5. Generate test code and verify it.

> [FIXME] Need auto generation.
