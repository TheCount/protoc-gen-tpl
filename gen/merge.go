package gen

import (
	"errors"
	"fmt"
	"sort"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// mergeData merges the data from the specified files into target.
func mergeData(
	target protoreflect.Message,
	msgxt protoreflect.ExtensionType, msgFields []protoreflect.Name,
) error {
	// Create deterministic file order.
	fds := make([]protoreflect.FileDescriptor, 0,
		protoregistry.GlobalFiles.NumFiles())
	protoregistry.GlobalFiles.RangeFiles(
		func(fd protoreflect.FileDescriptor) bool {
			fds = append(fds, fd)
			return true
		},
	)
	sort.Slice(fds, func(i, j int) bool {
		return fds[i].Path() < fds[j].Path()
	})
	// merge file data
	for _, fd := range fds {
		if err := mergeDataFromFile(target, fd, msgxt, msgFields); err != nil {
			return fmt.Errorf("merge from file '%s': %w", fd.Path(), err)
		}
	}
	return nil
}

// mergeDataFromFile merges the data from the specified file into target.
func mergeDataFromFile(
	target protoreflect.Message, fd protoreflect.FileDescriptor,
	msgxt protoreflect.ExtensionType, msgFields []protoreflect.Name,
) error {
	mds := fd.Messages()
	for i := 0; i != mds.Len(); i++ {
		md := mds.Get(i)
		if err := mergeDataFromMsg(target, md, msgxt, msgFields); err != nil {
			return fmt.Errorf("merge from message '%s': %w", md.FullName(), err)
		}
	}
	return nil
}

// mergeDataFromMsg merges the data from the specified message into target.
func mergeDataFromMsg(
	target protoreflect.Message, md protoreflect.MessageDescriptor,
	msgxt protoreflect.ExtensionType, msgFields []protoreflect.Name,
) (err error) {
	// process nested messages first
	mds := md.Messages()
	for i := 0; i != mds.Len(); i++ {
		submd := mds.Get(i)
		if err := mergeDataFromMsg(target, submd, msgxt, msgFields); err != nil {
			return fmt.Errorf("merge from nested message '%s': %w",
				submd.FullName(), err)
		}
	}
	// now process options
	msgOpt := md.Options()
	var xtMsg protoreflect.Message
	if proto.HasExtension(msgOpt, msgxt) {
		xtMsg = proto.GetExtension(msgOpt, msgxt).(protoreflect.Message)
	} else {
		// Extension might hide in unknown fields
		if xtMsg, err = extractUnknown(
			msgOpt.ProtoReflect().GetUnknown(), msgxt,
		); err != nil {
			return fmt.Errorf("extract option from unknown fields: %w", err)
		}
		if xtMsg == nil {
			return nil
		}
	}
	return mergeDataFromOpt(target, xtMsg, msgFields)
}

// mergeDataFromOpt merges the data from the specified option field into target.
func mergeDataFromOpt(
	target, opt protoreflect.Message, msgFields []protoreflect.Name,
) error {
	if len(msgFields) == 0 {
		return mergeMsg(target, opt)
	}
	field := opt.Descriptor().Fields().ByName(msgFields[0])
	if !opt.Has(field) {
		return nil
	}
	subopt := opt.Get(field).Interface().(protoreflect.Message)
	return mergeDataFromOpt(target, subopt, msgFields[1:])
}

// mergeMsg merges the given source message into the target message.
func mergeMsg(target, src protoreflect.Message) error {
	// Create deterministic range order
	type fdv struct {
		fd protoreflect.FieldDescriptor
		v  protoreflect.Value
	}
	var fdvs []fdv
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		fdvs = append(fdvs, fdv{fd, v})
		return true
	})
	sort.Slice(fdvs, func(i, j int) bool {
		return fdvs[i].fd.Number() < fdvs[j].fd.Number()
	})
	// Now merge
	for _, fdv := range fdvs {
		fd, v := fdv.fd, fdv.v
		// check if wrong oneof field is set in target before merging value
		oneof := fd.ContainingOneof()
		if oneof != nil {
			set := target.WhichOneof(oneof)
			if set != nil && set != fd {
				return fmt.Errorf(
					"unable to merge field '%s' value '%s' from oneof '%s' "+
						"in message '%s': field '%s' is set in target",
					fd.FullName(), v, oneof.FullName(),
					src.Type().Descriptor().FullName(), set.FullName(),
				)
			}
		}
		if err := mergeField(target, fd, v); err != nil {
			return fmt.Errorf("merge field '%s': %w", fd.FullName(), err)
		}
	}
	return nil
}

// mergeField merges the given value into target at the specified field
// descriptor.
func mergeField(
	target protoreflect.Message, fd protoreflect.FieldDescriptor,
	v protoreflect.Value,
) error {
	switch {
	case fd.IsList():
		return mergeList(target.Mutable(fd).List(), v.List())
	case fd.IsMap():
		return mergeMap(target.Mutable(fd).Map(), v.Map())
	}
	if target.Has(fd) {
		return errors.New("field already set")
	}
	target.Set(fd, v)
	return nil
}

// mergeList appends a shallow copy of the source list to the target list.
// Aliasing the source memory is OK in lists as the list elements themselves
// are never changed once appended.
func mergeList(target, src protoreflect.List) error {
	for i := 0; i != src.Len(); i++ {
		target.Append(src.Get(i))
	}
	return nil
}

// mergeMap merges the source map into the target map. If a key already exists
// in the target map, a recursive merge is attempted. This also means that a
// deep copy of the source elements has to be made, for later merges.
func mergeMap(target, src protoreflect.Map) (err error) {
	src.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch x := v.Interface().(type) {
		case protoreflect.Message:
			if err = mergeMsg(target.Mutable(k).Message(), x); err != nil {
				err = fmt.Errorf("merging map key '%s': %w", k, err)
				return false
			}
		default:
			if target.Has(k) {
				err = fmt.Errorf("map key '%s' already set in target", k)
				return false
			}
			target.Set(k, v)
		}
		return true
	})
	return
}

// extractUnknown attempts to extract a message of the specified extension
// type from the given concatenation of raw fields. If the field could not
// be found, extractUnknown returns (nil, nil).
func extractUnknown(
	rawFields []byte, xt protoreflect.ExtensionType,
) (protoreflect.Message, error) {
	xtidx := xt.TypeDescriptor().Number()
	for len(rawFields) > 0 {
		idx, typ, n := protowire.ConsumeField(rawFields)
		if n < 0 {
			return nil, fmt.Errorf("parsing raw field: %w", protowire.ParseError(n))
		}
		if idx != xtidx {
			rawFields = rawFields[n:]
			continue
		}
		if typ != protowire.BytesType {
			return nil, fmt.Errorf("bad type for extension message: %d", typ)
		}
		rawMsg, _ := protowire.ConsumeBytes(rawFields[protowire.SizeTag(idx):])
		msg := xt.New().Message().Interface()
		if err := proto.Unmarshal(rawMsg, msg); err != nil {
			return nil, fmt.Errorf("unmarshal extension message: %w", err)
		}
		return msg.ProtoReflect(), nil
	}
	return nil, nil
}
