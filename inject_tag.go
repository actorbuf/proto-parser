package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
	"regexp"
	"strings"
)

func injectTagMessage(msg *proto.Message) {
	for _, element := range msg.Elements {
		if field, ok := element.(*proto.NormalField); ok {
			if field.Comment == nil {
				continue
			}
			field.Comment.Lines = mergeInjectTag(field.Comment.Lines)
		}

		if field, ok := element.(*proto.Message); ok {
			injectTagMessage(field)
		}
	}
}

func mergeInjectTag(doc []string) []string {
	var res []string
	if len(doc) == 0 {
		return res
	}

	// 去重map
	var dupMap = make(map[string]string)
	var hasTag bool
	var newLineTag = "@gotags:"
	for _, d := range doc {
		if strings.Contains(d, "@gotags:") {
			hasTag = true
			var reg = regexp.MustCompile(`@gotags:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				val := rules[0][1]
				valkey := strings.Split(val, ":")[0]
				if _, ok := dupMap[valkey]; !ok {
					newLineTag = fmt.Sprintf("%s %s", newLineTag, rules[0][1])
				}
				dupMap[valkey] = val
				continue
			}
			continue
		}
		res = append(res, d)
	}
	if hasTag {
		res = append(res, newLineTag)
	}
	return res
}
