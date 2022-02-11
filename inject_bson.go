package proto_parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/emicklei/proto"
	log "github.com/sirupsen/logrus"
)

// injectMongoModelTag 遍历 msg 的字段 取出 comment 进行tag注入
func injectMongoModelTag(msg *proto.Message) {
	for _, m := range msg.Elements {
		if field, ok := m.(*proto.NormalField); ok {
			if field.Comment == nil {
				field.Comment = new(proto.Comment)
			}
			field.Comment.Lines = injectBsonTag(field, field.Name, field.Comment.Lines)

			if !isBuiltInType(field.Type) {
				// 非内建类型 找上级msg 看是否在allMsg中 不在 则直接找 全局m.Name
				var trackList []string
				var nowPath []string
				fatherDir := getInnerForefathersNameArr(msg)
				for _, np := range fatherDir {
					nowPath = append(nowPath, np)
					trackList = append(trackList, strings.Join(nowPath, "_"))
				}
				// 最近原则
				var prefix string
				for i := len(trackList) - 1; i >= 0; i-- {
					prefix = trackList[i] + "_" + field.Type
					if pm, exist := Visitor.AllMsgMap[prefix]; exist {
						injectMongoModelTag(pm)
						break
					}
					continue
				}

				if pm, exist := Visitor.AllMsgMap[field.Type]; exist {
					injectMongoModelTag(pm)
					continue
				}
			}
		}

		if field, ok := m.(*proto.Message); ok {
			injectMongoModelTag(field)
		}
	}
}

// injectTag 注入bson tag
func injectBsonTag(field *proto.NormalField, fieldName string, doc []string) []string {
	fieldName = toTitle(fieldName)
	gener := getOuterForefathersNameArr(field.Parent.(*proto.Message))
	gener = append(gener, fieldName)
	var newGender []string
	for _, node := range gener {
		newGender = append(newGender, case2Camel(node))
	}
	prefix := strings.Join(newGender, "_")
	var result []string

	if len(doc) == 0 {
		var val = calm2CaseBSON(fieldName)
		result = append(result, fmt.Sprintf("@gotags: bson:\"%s\"", val))
		Visitor.AddBsonTag(prefix, val)
		addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, val, trim(getInlineComment(field)))
		return result
	}

	var isInject bool //是否已经注入
	for _, t := range doc {
		if !isInject {
			var bsonReg = regexp.MustCompile(RegexpBson)
			res := bsonReg.FindAllStringSubmatch(t, -1)
			if len(res) >= 1 {
				if strings.ToLower(res[0][2]) == "ignore" {
					Visitor.AddBsonTag(prefix, field.Name)
					addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, field.Name, trim(getInlineComment(field)))
					return doc
				}
				t = strings.Replace(t, "@bson", "@gotags", 1)
				t = strings.Replace(t, res[0][2], fmt.Sprintf("bson:\"%s\"", res[0][2]), 1)
				Visitor.AddBsonTag(prefix, res[0][2])
				addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, res[0][2], trim(getInlineComment(field)))
				// 匹配上了 只替换第一个bson 多个bson配置取最开始一个
				result = append(result, t)
				isInject = true
				continue
			}
		}
		result = append(result, t)
	}

	// 默认注入小驼峰 但是需要注意 Id/ID 等价
	if !isInject {
		var val = calm2CaseBSON(fieldName)
		result = append(result, fmt.Sprintf("@gotags: bson:\"%s\"", val))
		//bsonTagFieldMap[prefix] = val
		Visitor.AddBsonTag(prefix, val)
		addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, val, trim(getInlineComment(field)))
	}

	return result
}

func addBsonFieldToMap(m *proto.Message, modelName, fieldName, prefix, value, inlineComment string) {
	if Visitor.ModelFieldStructMap == nil {
		Visitor.ModelFieldStructMap = make(map[string]map[string]ModelFieldStruct)
	}

	if _, exist := Visitor.ModelFieldStructMap[modelName]; !exist {
		Visitor.ModelFieldStructMap[modelName] = make(map[string]ModelFieldStruct)
	}

	prefix = strings.ReplaceAll(prefix, modelName+"_", "")

	Visitor.ModelFieldStructMap[modelName][prefix] = ModelFieldStruct{
		StructFieldName: case2Camel(fieldName),
		DbFieldName:     value,
		Comment:         inlineComment,
	}
}

// injectBsonMessage 如果message 开启了bson注入 则全字段注入
func injectBsonMessage(m *proto.Message) {
	if m.Comment == nil {
		return
	}
	if len(m.Comment.Lines) == 0 {
		return
	}
	var bsonReg = regexp.MustCompile(RegexpBson)
	for _, doc := range m.Comment.Lines {
		result := bsonReg.FindAllStringSubmatch(doc, -1)

		if len(result) == 0 {
			continue
		}

		if len(result[0]) < 3 {
			continue
		}

		if result[0][2] != "true" {
			continue
		}
		injectBsonMessageTag(m)
	}
}

func injectBsonMessageTag(msg *proto.Message) {
	for _, m := range msg.Elements {
		if field, ok := m.(*proto.NormalField); ok {
			if field.Comment == nil {
				field.Comment = new(proto.Comment)
			}
			field.Comment.Lines = injectMsgBsonTag(field, field.Name, field.Comment.Lines)

			if !isBuiltInType(field.Type) {
				// 非内建类型 找上级msg 看是否在allMsg中 不在 则直接找 全局m.Name
				var trackList []string
				var nowPath []string
				fatherDir := getOuterForefathersNameArr(msg)
				for _, np := range fatherDir {
					nowPath = append(nowPath, np)
					trackList = append(trackList, strings.Join(nowPath, "_"))
				}
				// 最近原则
				var prefix string
				for i := len(trackList) - 1; i >= 0; i-- {
					prefix = trackList[i] + "_" + field.Type
					if pm, exist := Visitor.AllMsgMap[prefix]; exist {
						injectBsonMessageTag(pm)
						break
					}
					continue
				}

				if pm, exist := Visitor.AllMsgMap[field.Type]; exist {
					injectBsonMessageTag(pm)
					continue
				}
			}
		}

		if field, ok := m.(*proto.Message); ok {
			injectBsonMessage(field)
		}
	}
}

func injectMsgBsonTag(field *proto.NormalField, fieldName string, doc []string) []string {
	fieldName = toTitle(fieldName)
	gener := getOuterForefathersNameArr(field.Parent.(*proto.Message))
	gener = append(gener, fieldName)
	var newGender []string
	for _, node := range gener {
		newGender = append(newGender, case2Camel(node))
	}
	prefix := strings.Join(newGender, "_")
	var result []string

	if len(doc) == 0 {
		var val = calm2CaseBSON(fieldName)
		result = append(result, fmt.Sprintf("@gotags: bson:\"%s\"", val))
		return result
	}

	var isInject bool //是否已经注入
	for _, t := range doc {
		if !isInject {
			var bsonReg = regexp.MustCompile(RegexpBson)
			res := bsonReg.FindAllStringSubmatch(t, -1)
			if len(res) >= 1 {
				if strings.ToLower(res[0][2]) == "ignore" {
					log.Infof("get tag prefix: %s, value: %s", prefix, field.Name)
					return doc
				}
				t = strings.Replace(t, "@bson", "@gotags", 1)
				t = strings.Replace(t, res[0][2], fmt.Sprintf("bson:\"%s\"", res[0][2]), 1)
				//log.Infof("get tag prefix: %s, value: %s", prefix, res[0][2])
				// 匹配上了 只替换第一个bson 多个bson配置取最开始一个
				result = append(result, t)
				isInject = true
				continue
			}
		}
		result = append(result, t)
	}

	// 默认注入小驼峰 但是需要注意 Id/ID 等价
	if !isInject {
		var val = calm2CaseBSON(fieldName)
		result = append(result, fmt.Sprintf("@gotags: bson:\"%s\"", val))
		//bsonTagFieldMap[prefix] = val
	}

	return result
}
