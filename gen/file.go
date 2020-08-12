package gen

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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
	if err := registerFiles(req.GetProtoFile()); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("register proto files: %w", err)
	}
	msgxt, data, err := getExtensions(params.Options)
	if err != nil {
		return nil, fmt.Errorf("get extension types: %w", err)
	}
	if err = mergeData(
		data, msgxt, params.Options.Message.Subfields,
	); err != nil {
		return nil, err
	}
	rawData := makeRawMessage(data)
	for key, value := range params.Extra {
		if rawData[key] != nil {
			return nil,
				fmt.Errorf("extra data key '%s' already present in proto data", key)
		}
		rawData[key] = value
	}
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

// registerFiles registers the specified proto files with the global registry.
func registerFiles(fdpbs []*descriptorpb.FileDescriptorProto) error {
	for _, fdpb := range fdpbs {
		fd, err := protodesc.NewFile(fdpb, protoregistry.GlobalFiles)
		if err != nil {
			return fmt.Errorf("create file descriptor for '%s': %w",
				fdpb.GetName(), err)
		}
		_, err = protoregistry.GlobalFiles.FindFileByPath(fd.Path())
		if err == nil {
			continue
		}
		if err != protoregistry.NotFound {
			return fmt.Errorf("looking up file '%s': %w", fd.Path(), err)
		}
		if err = protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
			return fmt.Errorf("register file '%s': %w", fd.Path(), err)
		}
		if err = registerTypesFromFile(fd); err != nil {
			return fmt.Errorf("register types for '%s': %w", fd.Path(), err)
		}
	}
	return nil
}

// registerTypesFromFile registers the types from the specified file.
func registerTypesFromFile(fd protoreflect.FileDescriptor) error {
	if err := registerEnums(fd.Enums()); err != nil {
		return fmt.Errorf("register enums: %w", err)
	}
	if err := registerMessages(fd.Messages()); err != nil {
		return fmt.Errorf("register messages: %w", err)
	}
	if err := registerExtensions(fd.Extensions()); err != nil {
		return fmt.Errorf("register extensions: %w", err)
	}
	return nil
}

// registerEnums registers the specified enums.
func registerEnums(eds protoreflect.EnumDescriptors) error {
	for i := 0; i != eds.Len(); i++ {
		ed := eds.Get(i)
		_, err := protoregistry.GlobalTypes.FindEnumByName(ed.FullName())
		if err == nil {
			continue
		}
		if err != protoregistry.NotFound {
			return fmt.Errorf("find enum '%s': %w", ed.FullName(), err)
		}
		et := dynamicpb.NewEnumType(ed)
		if err = protoregistry.GlobalTypes.RegisterEnum(et); err != nil {
			return fmt.Errorf("register enum '%s': %w", ed.FullName(), err)
		}
	}
	return nil
}

// registerMessages registers the specified messages.
func registerMessages(mds protoreflect.MessageDescriptors) error {
	for i := 0; i != mds.Len(); i++ {
		md := mds.Get(i)
		if err := registerEnums(md.Enums()); err != nil {
			return fmt.Errorf("register message '%s' enums: %w", md.FullName(), err)
		}
		if err := registerMessages(md.Messages()); err != nil {
			return fmt.Errorf("register message '%s' messages: %w",
				md.FullName(), err)
		}
		if err := registerExtensions(md.Extensions()); err != nil {
			return fmt.Errorf("register message '%s' extensions: %w",
				md.FullName(), err)
		}
		_, err := protoregistry.GlobalTypes.FindMessageByName(md.FullName())
		if err == nil {
			continue
		}
		if err != protoregistry.NotFound {
			return fmt.Errorf("find message '%s': %w", md.FullName(), err)
		}
		mt := dynamicpb.NewMessageType(md)
		if err = protoregistry.GlobalTypes.RegisterMessage(mt); err != nil {
			return fmt.Errorf("register message '%s': %w", md.FullName(), err)
		}
	}
	return nil
}

// registerExtensions registers the specified extensions.
func registerExtensions(xds protoreflect.ExtensionDescriptors) error {
	for i := 0; i != xds.Len(); i++ {
		xd := xds.Get(i)
		_, err := protoregistry.GlobalTypes.FindExtensionByName(xd.FullName())
		if err == nil {
			continue
		}
		if err != protoregistry.NotFound {
			return fmt.Errorf("find extension '%s': %w", xd.FullName(), err)
		}
		xt := dynamicpb.NewExtensionType(xd)
		if err = protoregistry.GlobalTypes.RegisterExtension(xt); err != nil {
			return fmt.Errorf("register extension '%s': %w", xd.FullName(), err)
		}
	}
	return nil
}
