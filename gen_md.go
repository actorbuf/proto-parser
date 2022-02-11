package proto_parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/elliotchance/pie/pie"
	"github.com/emicklei/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/sirupsen/logrus"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
)

var (
	srvName string
	rpcName string
)

// OutputMD 输出markdown文档
func OutputMD(pbFile, srv, rpc string, includes []string) error {
	reader, err := os.Open(pbFile)
	if err != nil {
		logrus.Errorf("open pb file: %+v, get err: %+v", pbFile, err)
		return err
	}
	defer reader.Close()
	parser := proto.NewParser(reader)
	definition, err := parser.Parse()
	if err != nil {
		logrus.Errorf("parse pb file err: %+v", err)
		return err
	}

	// 写入依赖message
	includes = append(includes, pbFile)
	includes = pie.Strings(includes).Unique()
	for _, include := range includes {
		reader, err := os.Open(include)
		if err != nil {
			logrus.Errorf("open pb file: %+v, get err: %+v", pbFile, err)
			return err
		}
		parser := proto.NewParser(reader)
		definition, err := parser.Parse()
		if err != nil {
			logrus.Errorf("parse pb file err: %+v", err)
			return err
		}
		// 拿包名
		proto.Walk(definition, proto.WithPackage(mdLoadPackage))

		// 扫一拨所有message
		proto.Walk(definition,
			proto.WithEnum(getAllEnum),
			proto.WithMessage(mdLoadAndGetAllMessage),
		)

		Visitor.MDocDepPkgName = ""
		_ = reader.Close()
	}

	srvName = srv
	rpcName = rpc

	// 第一轮 数据初始化
	proto.Walk(definition,
		proto.WithPackage(loadPackage),
		proto.WithEnum(getAllEnum),
		proto.WithService(parseSrvAndGetMsg),
	)

	// 判断是否存在
	if Visitor.MDoc == nil {
		return fmt.Errorf("not found srv or rpc")
	}

	if Visitor.MDoc.Node == nil {
		return fmt.Errorf("not found srv or rpc")
	}

	if Visitor.MDoc.ReqName == "" {
		return fmt.Errorf("not found request message")
	}

	if Visitor.MDoc.RspName == "" {
		return fmt.Errorf("not found response message")
	}

	// 获取请求体 响应体
	proto.Walk(definition, proto.WithMessage(getSrvMsg))

	if Visitor.MDoc.Req == nil {
		return fmt.Errorf("not found request message")
	}

	if Visitor.MDoc.Rsp == nil {
		return fmt.Errorf("not found response message")
	}

	getSrvMsgDetail(Visitor.MDoc.Req)
	getSrvMsgDetail(Visitor.MDoc.Rsp)

	// 输出 req
	reqBody, err := pbMsgToJSON(pbFile, fmt.Sprintf("%s.%s", Visitor.PackageName, Visitor.MDoc.ReqName))
	if err != nil {
		logrus.Errorf("proto message to json err: %+v", err)
		return err
	}
	Visitor.MDoc.ReqBody = string(reqBody)

	// 输出 resp
	respBody, err := pbMsgToJSON(pbFile, fmt.Sprintf("%s.%s", Visitor.PackageName, Visitor.MDoc.RspName))
	if err != nil {
		logrus.Errorf("proto message to json err: %+v", err)
		return err
	}
	Visitor.MDoc.RespBody = string(respBody)

	// 注册错误码
	if len(Visitor.MDoc.ErrCodeMap) != 0 && Visitor.ErrCodeEnum != nil {
		var otherErrCodeList MDocsErrCodes
		for key := range Visitor.MDoc.ErrCodeMap {
			if v, ok := Visitor.ErrCodeEnumFieldMap[key]; ok {
				Visitor.MDoc.ErrCodeList = append(Visitor.MDoc.ErrCodeList, MDocsErrCodeField{
					Code: v.Code,
					Name: key,
					Desc: v.Desc,
				})
			} else {
				name := fmt.Sprintf("%s.%s", Visitor.PackageName, key)
				if vv, ook := Visitor.ErrCodeEnumFieldMap[name]; ook {
					Visitor.MDoc.ErrCodeList = append(Visitor.MDoc.ErrCodeList, MDocsErrCodeField{
						Code: vv.Code,
						Name: key,
						Desc: vv.Desc,
					})
				} else {
					otherErrCodeList = append(otherErrCodeList, MDocsErrCodeField{
						Name: key,
					})
				}
			}
		}
		Visitor.MDoc.ErrCodeList = Visitor.MDoc.ErrCodeList.SortStableUsing(func(a, b MDocsErrCodeField) bool {
			return a.Code < b.Code
		})
		if len(otherErrCodeList) != 0 {
			Visitor.MDoc.ErrCodeList = append(Visitor.MDoc.ErrCodeList, otherErrCodeList...)
		}
	}

	t := template.New("md")
	t.Funcs(template.FuncMap{
		"length":         Len,
		"is_body_empty":  IsBodyEmpty,
		"not_body_empty": NotBodyEmpty,
	})
	t, err = t.Parse(OutputMDTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	//t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, Visitor.MDoc); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "==========\n%s", buf.String())

	return nil
}

func parseSrvAndGetMsg(srv *proto.Service) {
	if srv.Name != srvName {
		return
	}

	if srv.Comment == nil {
		return
	}

	if len(srv.Comment.Lines) == 0 {
		return
	}

	var doc = srv.Comment.Lines

	for _, com := range doc {
		var gr = regexp.MustCompile(RegexpGroupRouter)
		res := gr.FindAllStringSubmatch(com, -1)
		if len(res) == 0 {
			continue
		}
		if len(res[0]) != 2 {
			continue
		}

		if res[0][1] != "true" {
			return
		}

		// 是一个路由组
		genMD(srv)
		return
	}
}

func genMD(srv *proto.Service) {
	var node = new(GroupRouterNode)
	var apiPrefix string
	var reqName, rspName string
	// 当开启了自定义组前缀
	var doc = srv.Comment.Lines
	for _, com := range doc {
		var gra = regexp.MustCompile(RegexpGroupRouterAPI)
		res := gra.FindAllStringSubmatch(com, -1)
		if len(res) == 1 && len(res[0]) == 2 {
			apiPrefix = res[0][1]
		}
	}

	for _, rpc := range srv.Elements {
		sv := rpc.(*proto.RPC)
		if sv.Name != rpcName {
			continue
		}

		reqName = sv.RequestType
		rspName = sv.ReturnsType

		node = &GroupRouterNode{
			FuncName: sv.Name,
		}
		comment := sv.Comment.Lines
		getRpcErrCodeMap(comment)
		var isDefaultAPI, isDefaultMethod = true, true
		for _, line := range comment {
			// 检测author
			if strings.Contains(line, "@author:") {
				reg := regexp.MustCompile(RegexpRouterRpcAuthor)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Author = res[0][1]
					continue
				}
			}

			// 检测接口说明
			if strings.Contains(line, "@desc:") {
				reg := regexp.MustCompile(RegexpRouterRpcDesc)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Describe = res[0][1]
					continue
				}
			}

			// 检测接口使用方法
			if strings.Contains(line, "@method:") {
				reg := regexp.MustCompile(RegexpRouterRpcMethod)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					switch strings.ToUpper(res[0][1]) {
					case "GET":
						node.Method = "GET"
					case "POST":
						node.Method = "POST"
					case "DELETE":
						node.Method = "DELETE"
					case "PATCH":
						node.Method = "PATCH"
					case "OPTIONS":
						node.Method = "OPTIONS"
					case "PUT":
						node.Method = "PUT"
					case "ANY":
						node.Method = "ANY"
					}
					isDefaultMethod = false
					continue
				}
			}
			// 检测接口路由
			if strings.Contains(line, "@api") {
				reg := regexp.MustCompile(RegexpRouterRpcURL)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					prefix := res[0][1]
					if !strings.HasPrefix(prefix, "/") {
						prefix = fmt.Sprintf("/%s", prefix)
					}
					node.RouterPath = prefix
					isDefaultAPI = false
					continue
				}
			}
		}
		if isDefaultMethod {
			// 默认参数
			node.Method = "POST"
		}
		if isDefaultAPI {
			// 默认参数
			node.RouterPath = fmt.Sprintf("/%s", calm2Case(sv.Name))
		}
		break
	}

	if apiPrefix != "" {
		node.RouterPath = apiPrefix + node.RouterPath
	}
	if Visitor.MDoc == nil {
		Visitor.MDoc = &MDocs{
			Node:    node,
			ReqName: reqName,
			RspName: rspName,
		}
	} else {
		Visitor.MDoc.Node = node
		Visitor.MDoc.ReqName = reqName
		Visitor.MDoc.RspName = rspName
	}
}

func getAllEnum(e *proto.Enum) {
	if e.Name == "ErrCode" {
		Visitor.ErrCodeEnum = e
		getErrorCodeField(e)
	}
	Visitor.AddEnum(e.Name, e)
}

func getSrvMsg(msg *proto.Message) {
	Visitor.AddMsg(getOuterForefathersNameJoin(msg), msg)

	for _, elem := range msg.Elements {
		nestedMsg, ok := elem.(*proto.Message)
		if !ok {
			continue
		}
		getSrvMsg(nestedMsg)
	}

	if msg.Name == Visitor.MDoc.ReqName {
		Visitor.MDoc.Req = msg
		return
	}

	if msg.Name == Visitor.MDoc.RspName {
		Visitor.MDoc.Rsp = msg
	}
}

func getFieldDOC(msg *proto.Message, parentName string) []*MDocsField {
	if msg == nil {
		return nil
	}

	var typePrefix string
	if parentName != "" {
		typePrefix = parentName + "::"
	}

	var docField []*MDocsField
	for _, elem := range msg.Elements {
		var field *proto.NormalField
		var exist bool
		tPrefix := typePrefix
		if field, exist = elem.(*proto.NormalField); exist {
			var doc = new(MDocsField)
			if field.Repeated {
				tPrefix = fmt.Sprintf("Array::%s", tPrefix)
			}
			switch field.Type {
			case "uint32", "uint64", "int32", "int64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64":
				doc.FieldType = fmt.Sprintf("%sinteger", tPrefix)
			case "double", "float":
				doc.FieldType = fmt.Sprintf("%sfloat", tPrefix)
			case "string":
				doc.FieldType = fmt.Sprintf("%sstring", tPrefix)
			case "bool":
				doc.FieldType = fmt.Sprintf("%sbool", tPrefix)
			default:
				// 如果是枚举
				if e, ok := Visitor.AllEnumMap[field.Type]; ok {
					enumName := e.Name
					var edoc []*MDocsField
					for _, elem := range e.Elements {
						ee, assert := elem.(*proto.EnumField)
						if assert {
							//logrus.Infof("name: %+v, val: %+v", ee.Name, ee.Integer)
							var inlineMsg string
							if ee.InlineComment != nil {
								inlineMsg = ee.InlineComment.Message()
							}
							doc := &MDocsField{
								FieldName:  ee.Name,
								FieldDesc:  inlineMsg,
								FieldValue: ee.Integer,
							}
							edoc = append(edoc, doc)
						}
					}
					Visitor.AddDocEnum(enumName, edoc)
					doc.FieldType = fmt.Sprintf("%s%s(integer枚举)", tPrefix, field.Type)
				} else {
					doc.FieldType = fmt.Sprintf("%s%s(object对象)", tPrefix, field.Type)
					// 如果时外部导入的字段
					if strings.Contains(field.Type, ".") {
						t := strings.Split(field.Type, ".")
						if len(t) == 2 {
							pkg := t[0]
							name := t[1]
							if _, exist := Visitor.MDocDepMessageMap[pkg]; exist {
								if depMessage, exist1 := Visitor.MDocDepMessageMap[pkg][name]; exist1 {
									docField = append(docField, getFieldDOC(depMessage, field.Type)...)
								}
							}
						}
					}
				}
			}

			// 分析这个字段头部注释
			if field.Comment == nil || len(field.Comment.Lines) == 0 {
				doc.FieldName = calm2CaseBSON(field.Name)
				if field.InlineComment != nil && field.InlineComment.Message() != "" {
					doc.FieldDesc = strings.TrimLeft(field.InlineComment.Message(), " ")
				} else {
					doc.FieldDesc = "-"
				}
				docField = append(docField, doc)
				// 这个字段是否是内嵌msg
				if !isBuiltInType(field.Type) {
					realMsg, ok := Visitor.AllMsgMap[field.Type]
					if !ok {
						realMsg, ok = Visitor.AllMsgMap[fmt.Sprintf("%s_%s", getOuterForefathersNameJoin(msg), field.Type)]
						if !ok {
							continue
						}
					}
					docField = append(docField, getFieldDOC(realMsg, field.Type)...)
				}
				continue
			}

			coms := field.Comment.Lines

			for _, line := range coms {
				descReg := regexp.MustCompile(`@desc:\s*(.*)`)
				descRes := descReg.FindAllStringSubmatch(line, -1)
				if len(descRes) == 1 && len(descRes[0]) == 2 {
					doc.FieldDesc = descRes[0][1]
				} else {
					if field.InlineComment != nil && field.InlineComment.Message() != "" {
						doc.FieldDesc = strings.TrimLeft(field.InlineComment.Message(), " ")
					}
				}

				requireReg := regexp.MustCompile(`@v:\s*(.*)`)
				requireRes := requireReg.FindAllStringSubmatch(line, -1)
				if len(requireRes) == 1 && len(requireRes[0]) == 2 {
					rule := requireRes[0][1]
					if strings.Contains(rule, "required") {
						doc.IsRequire = true
					}
				}

				jsonReg := regexp.MustCompile(`@json:\s*(.*)`)
				jsonRes := jsonReg.FindAllStringSubmatch(line, -1)
				if len(jsonRes) == 1 && len(jsonRes[0]) == 2 {
					doc.FieldName = jsonRes[0][1]
					Visitor.AddDocJSONMap(field.Name, doc.FieldName)
				}
			}

			if len(coms) == 0 && field.InlineComment != nil && field.InlineComment.Message() != "" {
				doc.FieldDesc = strings.TrimLeft(field.InlineComment.Message(), " ")
			}

			if doc.FieldName == "" {
				doc.FieldName = calm2CaseBSON(field.Name)
			}

			// 这个字段是否是内嵌msg
			if !isBuiltInType(field.Type) {
				//logrus.Infof("comment father: %+v, field type: %+v", getOuterForefathersNameJoin(msg), field.Type)
				realMsg, ok := Visitor.AllMsgMap[field.Type]
				if !ok {
					realMsg = Visitor.AllMsgMap[fmt.Sprintf("%s_%s", getOuterForefathersNameJoin(msg), field.Type)]
				}
				docField = append(docField, getFieldDOC(realMsg, field.Type)...)
			}

			docField = append(docField, doc)
			continue
		}
	}

	return docField
}

func getSrvMsgDetail(msg *proto.Message) {
	docField := getFieldDOC(msg, "")

	if strings.HasSuffix(msg.Name, NameReq) {
		Visitor.MDoc.ReqFields = docField
		return
	}

	if strings.HasSuffix(msg.Name, NameResp) {
		Visitor.MDoc.RespFields = docField
		return
	}
}

// PbToJson 传入proto的数据，返回它对应的json数据
func pbMsgToJSON(protoPath, messageName string) ([]byte, error) {
	fd := getProtoFileDescriptor(protoPath)

	msg := fd.FindMessage(messageName)

	data := convertMessageToMap(msg)
	bs, err := json.MarshalIndent(data, "", "\t")
	return bs, err
}

func convertMessageToMap(message *desc.MessageDescriptor) map[string]interface{} {
	m := make(map[string]interface{})
	for _, fieldDescriptor := range message.GetFields() {
		fieldName := fieldDescriptor.GetName()
		if realName, exist := Visitor.MDoc.FieldJSONMap[fieldName]; exist {
			fieldName = realName
		}
		switch fieldDescriptor.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
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

func getProtoFileDescriptor(path string) *desc.FileDescriptor {
	p := protoparse.Parser{}
	fds, err := p.ParseFiles(path)
	if err != nil {
		logrus.Errorf("getProto ParseFiles error:%v", err)
		return nil
	}
	fd := fds[0]

	return fd
}

func mdLoadAndGetAllMessage(m *proto.Message) {
	Visitor.AddMDocDepMessage(Visitor.MDocDepPkgName, m)
}

func mdLoadPackage(p *proto.Package) {
	pkgName := strings.ReplaceAll(p.Name, ".", "_")
	Visitor.MDocDepPkgName = pkgName
}
