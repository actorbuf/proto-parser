package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
	"regexp"
	"strings"
)

// injectValidatorTag 注入tag
func injectMsgValidatorTag(msg *proto.Message) {
	for _, element := range msg.Elements {
		if field, ok := element.(*proto.NormalField); ok {
			if field.Comment == nil {
				continue
			}
			field.Comment.Lines = injectValidatorTag(field.Comment.Lines)
		}

		if field, ok := element.(*proto.Message); ok {
			injectMsgValidatorTag(field)
		}
	}
}

func injectValidatorTag(doc []string) []string {
	var res []string
	if len(doc) == 0 {
		return res
	}
	for _, d := range doc {
		if strings.Contains(d, "@v:") {
			var reg = regexp.MustCompile(`@v:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@v", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("binding:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			continue
		}
		res = append(res, d)
	}
	return res
}
