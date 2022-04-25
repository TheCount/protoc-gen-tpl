package gen

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// origMsg is the message key to the original protobuf message.
const origMsg = "_protomsg"

// enumValue describes an enum value as a string.
type enumValue string

// String simply converts this enum value to a string.
func (ev enumValue) String() string {
	return string(ev)
}

// kvpair describes a key-value pair.
type kvpair struct {
	key, value interface{}
}

// message is a data-only representation of a protobuf message.
type message map[string]interface{}

// String renders this message as a protobuf string.
func (m message) String() string {
	return m.string("")
}

// string renders this message as a string with the specified indendation.
func (m message) string(indent string) string {
	pairs := make([]kvpair, 0, len(m))
	for k, v := range m {
		if k == "" || k[0] == '_' || v == nil {
			continue
		}
		pairs = append(pairs, kvpair{k, v})
	}
	if len(pairs) == 0 {
		return "{}"
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key.(string) < pairs[j].key.(string)
	})
	pairStrings := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		pairStrings = append(pairStrings, fmt.Sprintf("%s  %s: %s",
			indent, pair.key, valueString(indent+"  ", pair.value)))
	}
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < len(pairStrings)-1; i++ {
		sb.WriteString(pairStrings[i])
		sb.WriteString(",\n")
	}
	sb.WriteString(pairStrings[len(pairStrings)-1])
	sb.WriteString("\n" + indent + "}")
	return sb.String()
}

// valueString renders the specified value as a protobuf string, with the
// specified indent.
func valueString(indent string, value interface{}) string {
	switch x := value.(type) {
	case message:
		return x.string(indent)
	case fmt.Stringer:
		return x.String()
	case string:
		return fmt.Sprintf("%q", x)
	case []byte:
		return fmt.Sprintf("%q", string(x))
	default:
		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Slice {
			return sliceString(indent, v)
		}
		if v.Kind() == reflect.Map {
			return mapString(indent, v)
		}
		return fmt.Sprintf("%v", value)
	}
}

// sliceString renders the specified slice value as a protobuf string, with the
// specified indent.
func sliceString(indent string, value reflect.Value) string {
	strs := make([]string, 0, value.Len())
	for i := 0; i != value.Len(); i++ {
		elem := value.Index(i)
		if elem.Kind() == reflect.Map && elem.IsNil() {
			// empty message
			continue
		}
		strs = append(strs, fmt.Sprintf("%s  %s",
			indent, valueString(indent+"  ", elem.Interface())))
	}
	if len(strs) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < len(strs)-1; i++ {
		sb.WriteString(strs[i])
		sb.WriteString(",\n")
	}
	sb.WriteString(strs[len(strs)-1])
	sb.WriteString("\n" + indent + "]")
	return sb.String()
}

// mapString renders the specified map value as a protobuf string, with the
// specified indent.
func mapString(indent string, value reflect.Value) string {
	pairs := make([]kvpair, 0, value.Len())
	vKeys := value.MapKeys()
	switch value.Type().Key().Kind() {
	case reflect.String:
		sort.Slice(vKeys, func(i, j int) bool {
			return vKeys[i].String() < vKeys[j].String()
		})
	case reflect.Int32, reflect.Int64:
		sort.Slice(vKeys, func(i, j int) bool {
			return vKeys[i].Int() < vKeys[j].Int()
		})
	case reflect.Uint32, reflect.Uint64:
		sort.Slice(vKeys, func(i, j int) bool {
			return vKeys[i].Uint() < vKeys[j].Uint()
		})
	}
	for _, vkey := range vKeys {
		elem := value.MapIndex(vkey)
		switch elem.Kind() {
		case reflect.Slice, reflect.Map:
			if elem.IsNil() {
				continue
			}
		}
		pairs = append(pairs, kvpair{vkey.Interface(), elem.Interface()})
	}
	if len(pairs) == 0 {
		return "{}"
	}
	pairStrings := make([]string, len(pairs))
	for _, pair := range pairs {
		if value.Type().Key().Kind() == reflect.String {
			pairStrings = append(pairStrings, fmt.Sprintf("%s  %q: %s",
				indent, pair.key, valueString(indent+"  ", pair.value)))
		} else {
			pairStrings = append(pairStrings, fmt.Sprintf("%s  %v: %s",
				indent, pair.key, valueString(indent+"  ", pair.value)))
		}
	}
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < len(pairStrings)-1; i++ {
		sb.WriteString(pairStrings[i])
		sb.WriteString(",\n")
	}
	sb.WriteString(pairStrings[len(pairStrings)-1])
	sb.WriteString("\n" + indent + "}")
	return sb.String()
}

// makeRawMessage converts the specified source message to a raw message.
func makeRawMessage(src protoreflect.Message) message {
	result := make(message)
	result[origMsg] = src.Interface()
	// Set all fields, including unpopulated ones (unless they're oneofs).
	fields := src.Descriptor().Fields()
	for i := 0; i != fields.Len(); i++ {
		fd := fields.Get(i)
		if !src.Has(fd) {
			switch {
			case fd.ContainingOneof() != nil: // omit this field
			case fd.Kind() == protoreflect.EnumKind:
				switch {
				case fd.IsList():
					result[string(fd.Name())] = []enumValue{}
				case fd.DefaultEnumValue() == nil:
					result[string(fd.Name())] =
						enumValue(fd.Enum().Values().ByNumber(fd.Default().Enum()).Name())
				default:
					result[string(fd.Name())] = enumValue(fd.DefaultEnumValue().Name())
				}
			default:
				result[string(fd.Name())] = reflect.Zero(getFieldType(fd)).Interface()
			}
			continue
		}
		v := src.Get(fd)
		switch {
		case fd.IsList():
			result[string(fd.Name())] = makeRawList(v.List())
		case fd.IsMap():
			result[string(fd.Name())] = makeRawMap(getFieldType(fd), v.Map())
		case fd.Kind() == protoreflect.EnumKind:
			result[string(fd.Name())] =
				enumValue(fd.Enum().Values().ByNumber(v.Enum()).Name())
		case fd.Kind() == protoreflect.MessageKind:
			result[string(fd.Name())] = makeRawMessage(v.Message())
		default:
			result[string(fd.Name())] = v.Interface()
		}
	}
	return result
}

// makeRawList converts the specified source list to a raw list.
func makeRawList(list protoreflect.List) []interface{} {
	result := make([]interface{}, list.Len())
	for i := range result {
		elem := list.Get(i)
		switch x := elem.Interface().(type) {
		case protoreflect.Message:
			result[i] = makeRawMessage(x)
		default:
			result[i] = elem.Interface()
		}
	}
	return result
}

// makeRawMap converts the specified source map to a map of the specified type.
func makeRawMap(mapType reflect.Type, m protoreflect.Map) interface{} {
	result := reflect.MakeMapWithSize(mapType, m.Len())
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch x := v.Interface().(type) {
		case protoreflect.Message:
			result.SetMapIndex(reflect.ValueOf(k.Interface()),
				reflect.ValueOf(makeRawMessage(x)))
		default:
			result.SetMapIndex(reflect.ValueOf(k.Interface()),
				reflect.ValueOf(v.Interface()))
		}
		return true
	})
	return result.Interface()
}

// getFieldType returns the Go type for the specified protobuf field type.
func getFieldType(field protoreflect.FieldDescriptor) reflect.Type {
	switch {
	case field.IsMap():
		k := getKindType(field.MapKey().Kind())
		v := getKindType(field.MapValue().Kind())
		return reflect.MapOf(k, v)
	case field.IsList():
		t := getKindType(field.Kind())
		return reflect.SliceOf(t)
	default:
		return getKindType(field.Kind())
	}
}

// getKindType returns the Go type for the specified protobuf kind.
func getKindType(kind protoreflect.Kind) reflect.Type {
	switch kind {
	default:
		return nil
	case protoreflect.BoolKind:
		return reflect.TypeOf(false)
	case protoreflect.EnumKind:
		return reflect.TypeOf(enumValue(""))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind:
		return reflect.TypeOf(int32(0))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return reflect.TypeOf(uint32(0))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind:
		return reflect.TypeOf(int64(0))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return reflect.TypeOf(uint64(0))
	case protoreflect.FloatKind:
		return reflect.TypeOf(float32(0))
	case protoreflect.DoubleKind:
		return reflect.TypeOf(float64(0))
	case protoreflect.StringKind:
		return reflect.TypeOf("")
	case protoreflect.BytesKind:
		return reflect.TypeOf([]byte{})
	case protoreflect.MessageKind:
		return reflect.TypeOf(message{})
	}
}
