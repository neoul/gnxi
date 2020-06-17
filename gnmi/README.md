# gNMI modeldata generation

1. Add YANG modules to `/yang` directory.
2. Updated `go generate` comment of the file in `gostruct` directory (e.g. `gostruct/gen.sample.go`).
3. Run `cd gostruct && go generate gen.sample.go` to generate the model structure.
4. Generate test code and verify it.
    - `Get` test code (done)
    - `Set` test code [FIXME]

> [FIXME] Need auto generation.
