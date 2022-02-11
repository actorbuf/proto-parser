package proto_parser

import (
	"github.com/emicklei/proto"
	"github.com/sirupsen/logrus"
	"regexp"
	"strconv"
	"strings"
)

// injectValidatorTag 注入tag
func collectIndex(msg *proto.Message) {
	if !strings.HasPrefix(msg.Name, NameModel) {
		return
	}

	for _, element := range msg.Elements {
		if field, ok := element.(*proto.NormalField); ok {
			if field.Comment == nil {
				continue
			}
			if err := doCollectIndex(field.Name, field.Comment.Lines); err != nil {
				logrus.Errorf("collect index err: %+v", err)
			}
		}

		// 目前先不支持设置子文档索引
		//if field, ok := element.(*proto.Message); ok {
		//	collectIndex(field)
		//}
	}
}

// doCollectIndex todo 取bson字段的field才行
func doCollectIndex(fieldName string, doc []string) error {
	if len(doc) == 0 {
		return nil
	}

	for _, d := range doc {
		if strings.Contains(d, "@index:") {
			var reg = regexp.MustCompile(`@index:\s*([\w]{5,})\s+(asc|desc|ASC|DESC)`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 3 {
				res := rules[0]
				if strings.ToLower(res[2]) == "asc" {
					if err := Visitor.AddIndexField(res[1], &IndexField{
						Field: fieldName,
						Sort:  1,
					}); err != nil {
						return err
					}
					continue
				}
				if err := Visitor.AddIndexField(res[1], &IndexField{
					Field: fieldName,
					Sort:  -1,
				}); err != nil {
					return err
				}
			}
			continue
		}

		if strings.Contains(d, "@unique_index:") {
			var reg = regexp.MustCompile(`@unique_index:\s*([\w]{5,})\s+(asc|desc|ASC|DESC)`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 3 {
				res := rules[0]
				if strings.ToLower(res[2]) == "asc" {
					if err := Visitor.AddUniqueIndexField(res[1], &IndexField{
						Field: fieldName,
						Sort:  1,
					}); err != nil {
						return err
					}
					continue
				}
				if err := Visitor.AddUniqueIndexField(res[1], &IndexField{
					Field: fieldName,
					Sort:  -1,
				}); err != nil {
					return err
				}
			}
			continue
		}
		if strings.Contains(d, "@ttl_index:") {
			var reg = regexp.MustCompile(`@ttl_index:\s*([\w]{5,})\s+(asc|desc|ASC|DESC)\s+(\d*)`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 4 {
				res := rules[0]
				if strings.ToLower(res[2]) == "asc" {
					if err := Visitor.AddTTLIndexField(res[1], string2int(res[3]), &IndexField{
						Field: fieldName,
						Sort:  1,
					}); err != nil {
						return err
					}
					continue
				}
				if err := Visitor.AddTTLIndexField(res[1], string2int(res[3]), &IndexField{
					Field: fieldName,
					Sort:  -1,
				}); err != nil {
					return err
				}
			}
			continue
		}
	}
	return nil
}

func string2int(src string) int64 {
	i, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		logrus.Errorf("parse str to int err: %+v", err)
		return 0
	}
	return i
}
