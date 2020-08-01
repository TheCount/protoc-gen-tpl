package gen

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// File generates a file from the specified code generator request.
func File(req *pluginpb.CodeGeneratorRequest) (
	*pluginpb.CodeGeneratorResponse_File, error,
) {
	if req.Parameter == nil {
		return nil, errors.New(parameterHelp)
	}
	params, err := parseParams(*req.Parameter)
	if err != nil {
		return nil, err
	}
	tpl, err := loadTemplate(params.TemplatePath)
	if err != nil {
		return nil, err
	}
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: req.GetProtoFile(),
	})
	if err != nil {
		return nil, fmt.Errorf("register proto files: %w", err)
	}
	msgxt, data, err := getExtensions(files, params.Options)
	if err != nil {
		return nil, fmt.Errorf("get extension types: %w", err)
	}
	if err = mergeData(
		data, files, msgxt, params.Options.Message.Subfields,
	); err != nil {
		return nil, err
	}
	rawData := makeRawMessage(data)
	var sb strings.Builder
	if err = tpl.Execute(&sb, rawData); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    &params.OutputPath,
		Content: proto.String(sb.String()),
	}, nil
}

// loadTemplate loads the template definition from the specified files.
func loadTemplate(glob string) (*template.Template, error) {
	tpl, err := template.ParseGlob(glob)
	if err != nil {
		return nil, fmt.Errorf("parse template pattern '%s': %w", glob, err)
	}
	return tpl, nil
}
