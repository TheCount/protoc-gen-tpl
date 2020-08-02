## protoc-gen-tpl

This is a protoc plugin to gather metadata (aka options) from protobuf source files and feed it into a template to generate arbitrary files.

Warning: this software is still in early alpha stage and has not been thoroughly tested.

## Installation

You should have `protoc` installed and a working [Go](https://golang.org) environment.

```sh
go get github.com/TheCount/protoc-gen-tpl/cmd/protoc-gen-tpl
```

In a terminal, run `protoc --tpl_out=. yourfile.proto` to get help on usage and options.
