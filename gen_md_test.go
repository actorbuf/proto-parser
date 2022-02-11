package proto_parser

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"testing"
)

func Test_convertMessageToMap(t *testing.T) {
	f := getProtoFileDescriptor("parser.proto")

	fd := f.FindMessage("proto_parser.ModelOperLog")

	if fd == nil {
		panic("fd nil")
	}

	var convertMessageToMap = func(message *desc.MessageDescriptor) map[string]interface{} {
		m := make(map[string]interface{})
		for _, fieldDescriptor := range message.GetFields() {
			fieldName := fieldDescriptor.GetName()
			switch fieldDescriptor.GetType() {
			case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
				fmt.Println("type message", fieldDescriptor.GetMessageType())
				if fieldDescriptor.IsRepeated() {
					// 如果是一个数组的话
					m[fieldName] = []interface{}{convertMessageToMap(fieldDescriptor.GetMessageType())}
					continue
				}
				m[fieldName] = convertMessageToMap(fieldDescriptor.GetMessageType())
				continue
			default:
				if fieldDescriptor.IsRepeated() {
					switch fieldDescriptor.GetType() {
					case descriptor.FieldDescriptorProto_TYPE_BOOL:
						m[fieldName] = []interface{}{false, true}
					case descriptor.FieldDescriptorProto_TYPE_STRING:
						m[fieldName] = []interface{}{""}
					default:
						m[fieldName] = []interface{}{0}
					}
					continue
				}
				m[fieldName] = fieldDescriptor.GetDefaultValue()
			}
		}
		return m
	}

	data := convertMessageToMap(fd)
	bs, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bs))
}
