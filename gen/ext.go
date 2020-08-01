package gen

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// getExtensions obtains the extension types for the option to provide the data.
// It also returns an empty data message.
func getExtensions(
	files *protoregistry.Files, options options,
) (msgxt protoreflect.ExtensionType, data protoreflect.Message, err error) {
	desc, err := files.FindDescriptorByName(options.Message.OptionFieldName)
	if err != nil {
		return nil, nil, fmt.Errorf("find message option descriptor '%s': %w",
			options.Message.OptionFieldName, err)
	}
	msgDesc, ok := desc.(protoreflect.ExtensionDescriptor)
	if !ok || !msgDesc.IsExtension() {
		return nil, nil, fmt.Errorf("not an extension field: %s",
			options.Message.OptionFieldName)
	}
	if msgDesc.ContainingMessage().FullName() !=
		"google.protobuf.MessageOptions" {
		return nil, nil,
			fmt.Errorf("not a message option: %s (containing message is '%s')",
				options.Message.OptionFieldName, msgDesc.ContainingMessage().FullName())
	}
	msgxt = dynamicpb.NewExtensionType(msgDesc)
	subDesc, err := getSubDescriptor(msgDesc, options.Message.Subfields)
	if err != nil {
		return nil, nil, fmt.Errorf("get subdescriptor: %w", err)
	}
	data = dynamicpb.NewMessage(subDesc).ProtoReflect()
	return
}

// getSubDescriptor returns the message descriptor of the subfield of the
// specified field descriptor determined by subfields.
func getSubDescriptor(
	fieldDesc protoreflect.FieldDescriptor, subfields []protoreflect.Name,
) (protoreflect.MessageDescriptor, error) {
	if fieldDesc.Kind() != protoreflect.MessageKind {
		return nil, fmt.Errorf("field '%s' is not a message", fieldDesc.FullName())
	}
	msgDesc := fieldDesc.Message()
	if len(subfields) == 0 {
		return msgDesc, nil
	}
	fieldDesc = msgDesc.Fields().ByName(subfields[0])
	if fieldDesc == nil {
		return nil, fmt.Errorf("message '%s' subfield '%s' not found",
			msgDesc.FullName(), subfields[0])
	}
	return getSubDescriptor(fieldDesc, subfields[1:])
}
