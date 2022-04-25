package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// parameterHelp contains the help text for the plugin parameters.
const parameterHelp = `
  Specify protoc-gen-tpl options as

    --tpl_out=key1=value1,key2=value2,…:output_dir

  The following keys are recognized:

  template
    Path to file template. The value can be a glob to specify multiple template
    files which define a template.
    See https://golang.org/pkg/text/template/ for template syntax.

  msgopt
    Message option to use as data input. The value must use protobuf syntax to
    specify the message option, i. e., the fully qualified message option field
    name, or, if an option submessage is to be used for data input, a value of
    the form

      (fully.qualified.message.option.field).subfield1.subfield2…

		Template data may contain additional fields starting with an underscore.
		These are currently for internal use only.

	extra=file.json
		Optional file with JSON data to provide as additional data to the template.

  out
    Path to output file.
`

// optionPath specifies a submessage within an option field.
type optionPath struct {
	// OptionFieldName is the fully qualified option field name.
	OptionFieldName protoreflect.FullName

	// Subfields is the (possibly empty) list of subfields.
	Subfields []protoreflect.Name
}

// String renders this option path as a string.
func (op optionPath) String() string {
	if len(op.Subfields) == 0 {
		return string(op.OptionFieldName)
	}
	var sb strings.Builder
	sb.WriteByte('(')
	sb.WriteString(string(op.OptionFieldName))
	sb.WriteByte(')')
	for _, subfield := range op.Subfields {
		sb.WriteByte('.')
		sb.WriteString(string(subfield))
	}
	return sb.String()
}

// Validate validates this option path.
// A nil option path is considered valid.
func (op *optionPath) Validate() error {
	if op == nil {
		return nil
	}
	if !op.OptionFieldName.IsValid() {
		return fmt.Errorf("option field name '%s' is invalid", op.OptionFieldName)
	}
	for _, subfield := range op.Subfields {
		if !subfield.IsValid() {
			return fmt.Errorf("invalid subfield '%s' in %s", subfield, op)
		}
	}
	return nil
}

// options describes option messages to use.
type options struct {
	// Message specifies the message option path to use.
	Message *optionPath
}

// Validate validates these options.
func (o *options) Validate() error {
	if o.Message == nil {
		return errors.New("no options specified")
	}
	if err := o.Message.Validate(); err != nil {
		return fmt.Errorf("message option path: %w", err)
	}
	return nil
}

// params describes the generator parameters.
type params struct {
	// TemplatePath is the path to the input template (glob).
	TemplatePath string

	// Options specifies which option messages to use as a basis for the data.
	Options options

	// Extra optionally contains extra data for the template.
	Extra map[string]interface{}

	// OutputPath is the path to the output file.
	OutputPath string
}

// Validate validates these params.
func (p *params) Validate() error {
	if p.TemplatePath == "" {
		return errors.New("template path is empty")
	}
	if p.OutputPath == "" {
		return errors.New("output path is empty")
	}
	return p.Options.Validate()
}

// parseParams parses the input string
func parseParams(in string) (*params, error) {
	var result params
	parts := strings.Split(in, ",")
	for _, part := range parts {
		idx := strings.Index(part, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid option '%s'", part)
		}
		switch part[:idx] {
		default:
			return nil, fmt.Errorf("unsupported option '%s'", part[:idx])
		case "template":
			result.TemplatePath = part[idx+1:]
		case "msgopt":
			path, err := parseOptionPath(part[idx+1:])
			if err != nil {
				return nil, fmt.Errorf("parse message option path '%s': %w",
					part[idx+1:], err)
			}
			result.Options.Message = path
		case "extra":
			filename := part[idx+1:]
			f, err := os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("open extra data file '%s': %w", filename, err)
			}
			defer f.Close()
			decoder := json.NewDecoder(f)
			if err = decoder.Decode(&result.Extra); err != nil {
				return nil, fmt.Errorf("decoding extra data file '%s': %w",
					filename, err)
			}
		case "out":
			result.OutputPath = part[idx+1:]
		}
	}
	return &result, result.Validate()
}

// parseOptionPath parses the specified input string as an option path.
func parseOptionPath(in string) (*optionPath, error) {
	if in == "" {
		return nil, errors.New("empty option path")
	}
	if in[0] != '(' {
		return &optionPath{
			OptionFieldName: protoreflect.FullName(in),
		}, nil
	}
	idx := strings.Index(in, ")")
	if idx < 0 {
		return nil, errors.New("missing ')' in option path")
	}
	result := &optionPath{
		OptionFieldName: protoreflect.FullName(in[1:idx]),
	}
	if len(in) == idx+1 {
		return result, nil
	}
	result.Subfields = protoreflectNames(strings.Split(in[idx+2:], "."))
	return result, nil
}

// protoreflectNames converts []string to []protoreflect.Name
func protoreflectNames(in []string) []protoreflect.Name {
	result := make([]protoreflect.Name, len(in))
	for i, str := range in {
		result[i] = protoreflect.Name(str)
	}
	return result
}
