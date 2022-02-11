package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
	"regexp"
	"strings"
)

// injectGormModelTag 遍历 msg 的字段 取出 comment 进行tag注入
func injectGormModelTag(msg *proto.Message) {
	for _, m := range msg.Elements {
		if field, ok := m.(*proto.NormalField); ok {
			if field.Comment == nil {
				field.Comment = new(proto.Comment)
			}
			field.Comment.Lines = injectGormTag(field, field.Name, field.Comment.Lines)

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
						injectGormModelTag(pm)
						break
					}
					continue
				}

				if pm, exist := Visitor.AllMsgMap[field.Type]; exist {
					injectGormModelTag(pm)
					continue
				}
			}
		}

		if field, ok := m.(*proto.Message); ok {
			injectGormModelTag(field)
		}
	}
}

// injectGormTag 注入gorm tag fieldName 用来生成 column标记
func injectGormTag(field *proto.NormalField, fieldName string, doc []string) []string {
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
		var val = GormTagGenerate(fieldName, "", field.Type)
		result = append(result, fmt.Sprintf("@gotags: gorm:\"%s\"", val))
		Visitor.AddBsonTag(prefix, val)
		addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, val, trim(getInlineComment(field)))
		return result
	}

	var isInject bool    //是否已经注入
	var injectDoc string // 被注入的文档内容
	for _, t := range doc {
		if !isInject {
			var bsonReg = regexp.MustCompile(RegexpGorm)
			res := bsonReg.FindAllStringSubmatch(t, -1)
			if len(res) >= 1 {
				injectDoc = t
				if strings.ToLower(res[0][2]) == "ignore" {
					Visitor.AddBsonTag(prefix, field.Name)
					addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, field.Name, trim(getInlineComment(field)))
					return doc
				}
				t = strings.Replace(t, "@gorm", "@gotags", 1)
				t = strings.Replace(t, res[0][2], fmt.Sprintf("gorm:\"%s\"", res[0][2]), 1)
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
		var val = GormTagGenerate(fieldName, injectDoc, field.Type)
		result = append(result, fmt.Sprintf("@gotags: gorm:\"%s\"", val))
		//bsonTagFieldMap[prefix] = val
		Visitor.AddBsonTag(prefix, val)
		addBsonFieldToMap(field.Parent.(*proto.Message), getModelName(field.Parent.(*proto.Message)), fieldName, prefix, val, trim(getInlineComment(field)))
	}

	return result
}
