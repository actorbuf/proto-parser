package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
	"regexp"
	"strings"
)

type Style int

const (
	underscore Style = iota
	lowerCamel
	upperCamel
	kebabCase
	raw

	underscoreVal = "underscore"
	lowerCamelVal = "lower_camel"
	upperCamelVal = "upper_camel"
	kebabCaseVal  = "kebab_case"
	rawVal        = "raw"
)

// injectMsgJsonTag 注入tag
func injectMsgJsonTag(msg *proto.Message) {
	if msg.Comment == nil {
		injectMsgJsonTagByStyle(msg, raw)
		return
	}

	if len(msg.Comment.Lines) == 0 {
		injectMsgJsonTagByStyle(msg, raw)
		return
	}

	regStyle := regexp.MustCompile(RegexpJsonStyle)

	for _, line := range msg.Comment.Lines {
		res := regStyle.FindAllStringSubmatch(line, -1)
		if len(res) == 0 {
			continue
		}

		if len(res[0]) != 2 {
			continue
		}

		val := res[0][1]

		switch val {
		case rawVal:
			injectMsgJsonTagByStyle(msg, raw)
			return
		case underscoreVal:
			injectMsgJsonTagByStyle(msg, underscore)
			return
		case lowerCamelVal:
			injectMsgJsonTagByStyle(msg, lowerCamel)
			return
		case upperCamelVal:
			injectMsgJsonTagByStyle(msg, upperCamel)
			return
		case kebabCaseVal:
			injectMsgJsonTagByStyle(msg, kebabCase)
		}
	}

	injectMsgJsonTagByStyle(msg, raw)
}

func injectMsgJsonTagByStyle(msg *proto.Message, style Style) {
	switch style {
	case raw:
		for _, element := range msg.Elements {
			if field, ok := element.(*proto.NormalField); ok {
				if field.Comment == nil {
					continue
				}
				field.Comment.Lines = injectRawJsonTag(field.Comment.Lines)
			}

			if field, ok := element.(*proto.Message); ok {
				injectMsgJsonTagByStyle(field, raw)
			}
		}
	case underscore:
		for _, element := range msg.Elements {
			if field, ok := element.(*proto.NormalField); ok {
				if field.Comment == nil {
					field.Comment = new(proto.Comment)
					field.Comment.Lines = injectUnderscoreJsonTag(field.Name, nil)
				}
				field.Comment.Lines = injectUnderscoreJsonTag(field.Name, field.Comment.Lines)
			}

			if field, ok := element.(*proto.Message); ok {
				injectMsgJsonTagByStyle(field, underscore)
			}
		}
	case lowerCamel:
		for _, element := range msg.Elements {
			if field, ok := element.(*proto.NormalField); ok {
				if field.Comment == nil {
					field.Comment = new(proto.Comment)
					field.Comment.Lines = injectLowerCamelJsonTag(field.Name, nil)
				}
				field.Comment.Lines = injectLowerCamelJsonTag(field.Name, field.Comment.Lines)
			}

			if field, ok := element.(*proto.Message); ok {
				injectMsgJsonTagByStyle(field, lowerCamel)
			}
		}
	case upperCamel:
		for _, element := range msg.Elements {
			if field, ok := element.(*proto.NormalField); ok {
				if field.Comment == nil {
					field.Comment = new(proto.Comment)
					field.Comment.Lines = injectUpperCamelJsonTag(field.Name, nil)
				}
				field.Comment.Lines = injectUpperCamelJsonTag(field.Name, field.Comment.Lines)
			}

			if field, ok := element.(*proto.Message); ok {
				injectMsgJsonTagByStyle(field, upperCamel)
			}
		}
	case kebabCase:
		for _, element := range msg.Elements {
			if field, ok := element.(*proto.NormalField); ok {
				if field.Comment == nil {
					field.Comment = new(proto.Comment)
					field.Comment.Lines = injectKebabCaseJsonTag(field.Name, nil)
				}
				field.Comment.Lines = injectKebabCaseJsonTag(field.Name, field.Comment.Lines)
			}

			if field, ok := element.(*proto.Message); ok {
				injectMsgJsonTagByStyle(field, kebabCase)
			}
		}
	}
}

// injectRawJsonTag 原样的json输出 只自定义部份参数
func injectRawJsonTag(doc []string) []string {
	var res []string
	if len(doc) == 0 {
		return res
	}
	for _, d := range doc {
		if strings.Contains(d, "@json:") {
			var reg = regexp.MustCompile(`@json:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@json", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("json:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			continue
		}
		res = append(res, d)
	}
	return res
}

// injectUnderscoreJsonTag 强制所有的json输出为下划线
func injectUnderscoreJsonTag(fieldName string, doc []string) []string {
	var force bool
	var res []string

	if len(doc) == 0 {
		goto INJECT
	}

	for _, d := range doc {
		if strings.Contains(d, "@json:") {
			var reg = regexp.MustCompile(`@json:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@json", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("json:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			force = true
			continue
		}
		res = append(res, d)
	}
INJECT:
	// 已经被强制变更过
	if force {
		return res
	}

	res = append(res, fmt.Sprintf("@gotags: json:\"%s\"", calm2Case(fieldName)))

	return res
}

// injectLowerCamelJsonTag 强制所有的json输出为小驼峰
func injectLowerCamelJsonTag(fieldName string, doc []string) []string {
	var force bool
	var res []string
	if len(doc) == 0 {
		goto INJECT
	}
	for _, d := range doc {
		if strings.Contains(d, "@json:") {
			var reg = regexp.MustCompile(`@json:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@json", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("json:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			continue
		}
		res = append(res, d)
	}
INJECT:
	if force {
		return res
	}
	res = append(res, fmt.Sprintf("@gotags: json:\"%s\"", case2LowerCamel(fieldName)))
	return res
}

// injectUpperCamelJsonTag 强制所有的json输出为大驼峰
func injectUpperCamelJsonTag(fieldName string, doc []string) []string {
	var force bool
	var res []string
	if len(doc) == 0 {
		goto INJECT
	}
	for _, d := range doc {
		if strings.Contains(d, "@json:") {
			var reg = regexp.MustCompile(`@json:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@json", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("json:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			continue
		}
		res = append(res, d)
	}

INJECT:
	// 已经被强制变更过
	if force {
		return res
	}
	res = append(res, fmt.Sprintf("@gotags: json:\"%s\"", case2Camel(fieldName)))
	return res
}

// injectKebabCaseJsonTag 强制所有的json输出为短横线
func injectKebabCaseJsonTag(fieldName string, doc []string) []string {
	var force bool
	var res []string
	if len(doc) == 0 {
		goto INJECT
	}
	for _, d := range doc {
		if strings.Contains(d, "@json:") {
			var reg = regexp.MustCompile(`@json:\s*(.*)\s?`)
			rules := reg.FindAllStringSubmatch(d, -1)
			if len(rules) > 0 && len(rules[0]) == 2 {
				d = strings.Replace(d, "@json", "@gotags", -1)
				d = strings.Replace(d, rules[0][1], fmt.Sprintf("json:\"%s\"", rules[0][1]), -1)
			}
			res = append(res, d)
			continue
		}
		res = append(res, d)
	}

INJECT:
	// 已经被强制变更过
	if force {
		return res
	}
	res = append(res, fmt.Sprintf("@gotags: json:\"%s\"", calm2KebabCaseBSON(fieldName)))
	return res
}
