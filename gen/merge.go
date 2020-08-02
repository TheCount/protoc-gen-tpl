package gen

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// mergeData merges the data from the specified files into target.
func mergeData(
	target protoreflect.Message,
	msgxt protoreflect.ExtensionType, msgFields []protoreflect.Name,
) (err error) {
	protoregistry.GlobalFiles.RangeFiles(
		func(fd protoreflect.FileDescriptor) bool {
			if err = mergeDataFromFile(target, fd, msgxt, msgFields); err != nil {
				err = fmt.Errorf("merge from file '%s': %w", fd.Path(), err)
				return false
			}
			return true
		},
	)
	return
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
) error {
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
	if !proto.HasExtension(msgOpt, msgxt) {
		return nil
	}
	xtMsg := proto.GetExtension(msgOpt, msgxt).(protoreflect.Message)
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
func mergeMsg(target, src protoreflect.Message) (err error) {
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		// check if wrong oneof field is set in target before merging value
		oneof := fd.ContainingOneof()
		if oneof != nil {
			set := target.WhichOneof(oneof)
			if set != nil && set != fd {
				err = fmt.Errorf(
					"unable to merge field '%s' value '%s' from oneof '%s' "+
						"in message '%s': field '%s' is set in target",
					fd.FullName(), v, oneof.FullName(),
					src.Type().Descriptor().FullName(), set.FullName(),
				)
				return false
			}
		}
		if err = mergeField(target, fd, v); err != nil {
			err = fmt.Errorf("merge field '%s': %w", fd.FullName(), err)
		}
		return err == nil
	})
	return err
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
