package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/TheCount/protoc-gen-tpl/gen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	resp := generate()
	buf, err := proto.Marshal(resp)
	if err != nil {
		log.Fatalf("Marshal CodeGeneratorResponse: %s", err)
	}
	if _, err := os.Stdout.Write(buf); err != nil {
		log.Fatalf("Write CodeGeneratorResponse: %s", err)
	}
}

// generate reads in the code generator request from stdin and creates the
// appropriate response.
func generate() *pluginpb.CodeGeneratorResponse {
	resp := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: proto.Uint64(uint64(
			pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		resp.Error = proto.String(fmt.Sprintf("read CodeGeneratorRequest: %s", err))
		return resp
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(buf, &req); err != nil {
		resp.Error = proto.String(fmt.Sprintf("unmarshal CodeGeneratorRequest: %s",
			err))
		return resp
	}
	f, err := gen.File(&req)
	if err != nil {
		resp.Error = proto.String(err.Error())
		return resp
	}
	resp.File = []*pluginpb.CodeGeneratorResponse_File{f}
	return resp
}
