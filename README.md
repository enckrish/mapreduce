Compile protobuf definitions into Go (library package)

This README explains how to compile the .proto definitions in `mr-protos/` into Go sources under the `library/` folder (the exact commands I used while working on this repo).

Checklist
- [ ] Install protoc (protobuf compiler)
- [ ] Install Go protoc plugins (protoc-gen-go and protoc-gen-go-grpc)
- [ ] Ensure `mr-protos` contains any well-known types you reference (or point protoc to their include dir)
- [ ] Run protoc to generate *.pb.go and *_grpc.pb.go into the `library/` folder
- [ ] Fetch Go module dependencies and build `library`

Prerequisites
1. protoc (protobuf compiler) installed and on PATH. Check with:

```bash
protoc --version
```

2. Go >= 1.20 (or the project's version). Ensure $GOBIN or $GOPATH/bin is on your PATH so the installed plugins are found by protoc.

3. Install the Go protoc plugins (examples use specific versions used when running earlier):

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
```

(These will install `protoc-gen-go` and `protoc-gen-go-grpc` to $GOBIN or $GOPATH/bin.)

Notes about well-known types (google/protobuf/*.proto)
- If your protos import well-known types such as `google/protobuf/empty.proto`, protoc needs to find them. Options:
  - Put a local copy under `mr-protos/google/protobuf/` (e.g. `mr-protos/google/protobuf/empty.proto`) so `--proto_path=mr-protos` resolves the import. This repo already contains that file in some setups.
  - Or add the path where the protobuf include files live to `--proto_path`.

Proto package mapping (go_package)
- Recommended: add `option go_package = "github.com/enckrish/library;library";` to each .proto so the generated code has the correct import path and package name.
- If you cannot change the .proto files, you can supply explicit mapping using the `M` flags (see example below).

A: Simple generation (if your .proto files already include `option go_package`)

```bash
protoc \
  --proto_path=mr-protos \
  --go_out=paths=source_relative:library \
  --go-grpc_out=paths=source_relative:library \
  mr-protos/*.proto
```

This will generate source-relative .pb.go and _grpc.pb.go files into the `library/` directory (preserving filename prefixes).

B: Generation with explicit mapping (if .proto files don't have `go_package` set)

The `M` mapping tells `protoc-gen-go` which Go import path to use for a given proto filename.

```bash
protoc --proto_path=mr-protos \
>   --go_out=paths=source_relative,Mcoordinator.proto=github.com/enckrish/library:library \
>   --go-grpc_out=paths=source_relative,Mcoordinator.proto=github.com/enckrish/library:library \
>   mr-protos/coordinator.proto
```

(Adjust the mapping keys `M<proto-filename>=<go-import-path>` for whichever proto files you compile.)

Post-generation: fetch deps & build

After generating Go sources under `library/`, change to the `library` module and fetch required dependencies and try building:

```bash
cd library
# fetch grpc & protobuf Go modules (choose versions that match your toolchain)
go get google.golang.org/grpc@v1.56.0 google.golang.org/protobuf@v1.30.0
# tidy and build
go mod tidy
go build ./...
```

Troubleshooting
- Error: `google/protobuf/empty.proto: File not found` — either add `mr-protos/google/protobuf/empty.proto` or include the directory that contains the well-known protos in `--proto_path`.
- Error: `Please specify either a "go_package" option` — add `option go_package = "github.com/enckrish/library;library";` to the proto, or use the `M` mapping shown above.
- Error: `undefined: grpc.SupportPackageIsVersion9` or similar — this indicates a mismatch between generated code expectations and the `google.golang.org/grpc` module version. Try upgrading the grpc module in `library` (e.g. `go get google.golang.org/grpc@v1.56.0`) and rebuild.
- If the generated gRPC code expects newer helper symbols, ensure `protoc-gen-go-grpc` version and your `google.golang.org/grpc` module version are compatible.

Re-generate when changing proto files
- Whenever you change `.proto` files you should re-run the protoc commands above to re-generate the Go sources.

If you want, I can:
- add `option go_package` to the .proto files in `mr-protos/` for you (recommended), and re-run the generation; or
- run the exact commands for you now and confirm `go build` succeeds.


