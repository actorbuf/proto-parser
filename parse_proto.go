package proto_parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	log "github.com/sirupsen/logrus"
	"github.com/actorbuf/iota/core"
)

func openProtoFile(pbFile string) (*proto.Proto, error) {
	reader, err := os.Open(pbFile)
	if err != nil {
		log.Errorf("open pb file: %+v, get err: %+v", pbFile, err)
		return nil, err
	}
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, err := parser.Parse()
	if err != nil {
		log.Errorf("parse pb file err: %+v", err)
		return nil, err
	}
	return definition, nil
}

// ParseProto 传入proto文件 返回中间proto文件和错误信息
func ParseProto(pbFile string) (midFile string, err error) {
	definition, err := openProtoFile(pbFile)
	if err != nil {
		return midFile, err
	}

	// 第一轮 数据初始化
	proto.Walk(definition,
		proto.WithImport(loadImportPackage),
		proto.WithPackage(loadPackage),
		proto.WithMessage(loadMessage),
		proto.WithService(injectFreqMap),
		proto.WithEnum(loadErrCodeEnum),
	)

	for _, message := range Visitor.ModelMsgMap {
		// 基于不同的数据库驱动 生成不同的代码
		switch Visitor.dbDriver {
		case "gdbc":
			injectGormModelTag(message)
		default:
			// 默认 mongodb 贴合一下小黑屋的技术栈
			injectMongoModelTag(message)
		}
		// 生成表
		genModelTableName(message)
	}

	// 注册非model的message 索引收集
	proto.Walk(definition,
		proto.WithMessage(injectBsonMessage),
		proto.WithMessage(collectIndex),
	)
	// 合并tag
	proto.Walk(definition, proto.WithMessage(injectTagMessage))

	baseName := strings.ReplaceAll(path.Base(pbFile), "origin_", "")
	dirName := path.Dir(pbFile)
	midFile = fmt.Sprintf("%s/%s", dirName, baseName)
	if err := parserFormatWrite(midFile, definition); err != nil {
		log.Errorf("parse pb file err: %+v", err)
		return midFile, err
	}

	if err := GenModelCode(Visitor.PackageName, midFile); err != nil {
		log.Errorf("err: %+v", err)
	}

	if err := parseProtoRouter(pbFile); err != nil {
		log.Errorf("err: %+v", err)
	}

	return
}

// parseProtoRouter 解析proto路由相关
func parseProtoRouter(pbFile string) error {
	definition, err := openProtoFile(pbFile)
	if err != nil {
		return err
	}

	PbFilePath = pbFile

	proto.Walk(definition,
		proto.WithImport(loadImportPackage),
		proto.WithPackage(loadPackage),
		proto.WithService(loadService),
		proto.WithEnum(loadErrCodeEnum),
	)

	// 注册路由组
	if err := genGroupRouterTemplate(pbFile); err != nil {
		log.Errorf("err: %+v", err)
	}

	// 注册rpc的错误码
	proto.Walk(definition, proto.WithService(checkRouterErrorCode))

	if err := parserFormatWrite(pbFile, definition); err != nil {
		log.Errorf("parse pb file err: %+v", err)
		return err
	}

	return nil
}

var Visitor *ProtoVisitor

func init() {
	if Visitor == nil {
		Visitor = &ProtoVisitor{}
	}
}

func loadImportPackage(pkg *proto.Import) {
	// todo
}

func loadService(srv *proto.Service) {
	parseSrvGenRouter(srv)
	parseSrvGenRPC(srv)
	// 生成task相关
	parseSrvGenTask(srv)
}

func loadPackage(p *proto.Package) {
	pkgName := strings.ReplaceAll(p.Name, ".", "_")
	Visitor.PackageName = pkgName
}

func loadMessage(m *proto.Message) {
	// 注册 validator
	injectMsgValidatorTag(m)

	// 注册 json
	injectMsgJsonTag(m)

	// 将该msg加入全局msg池子
	Visitor.AddMsg(getOuterForefathersNameJoin(m), m)

	// 找所有的model
	addInformalModelMsg(m)
}

// todo 这里需要对同级目录下的所有 ErrCode 进行合并
func loadErrCodeEnum(e *proto.Enum) {
	if e.Name != ErrCodeName {
		return
	}

	for _, obj := range e.Elements {
		field, ok := obj.(*proto.EnumField)
		if !ok {
			continue
		}
		if field.InlineComment == nil {
			Visitor.AddErrCode(field.Integer, trim(field.Name), trim(field.Name))
			continue
		}

		if len(field.InlineComment.Lines) == 0 {
			Visitor.AddErrCode(field.Integer, trim(field.Name), trim(field.Name))
			continue
		}
		Visitor.AddErrCode(field.Integer, trim(field.Name), trim(field.InlineComment.Message()))
	}
}

// addInformalModelMsg 添加非正式的message到model map中
// 需要依赖 @model: true
func addInformalModelMsg(msg *proto.Message) {
	if strings.HasPrefix(msg.Name, NameModel) {
		Visitor.AddModelMsg(msg.Name, msg)
		return
	}

	if msg.Comment == nil {
		return
	}
	if len(msg.Comment.Lines) == 0 {
		return
	}
	reg := regexp.MustCompile(RegexpAddModel)
	docs := msg.Comment.Lines
	for _, doc := range docs {
		if reg.MatchString(doc) {
			Visitor.AddModelMsg(msg.Name, msg)
		}
	}
}

// genModelTableName 收集表名
func genModelTableName(msg *proto.Message) {
	modelName := trim(msg.Name)

	var tableName string

	if msg.Comment == nil {
		tableName = calm2Case(modelName)
		tableName = strings.Replace(tableName, "model_", "", 1)
		Visitor.AddModelTableName(modelName, tableName)
		return
	}

	for _, doc := range msg.Comment.Lines {
		var reg = regexp.MustCompile(`@table_name:\s*(\w+)\s*`)
		res := reg.FindAllStringSubmatch(doc, -1)
		if len(res) == 1 && len(res[0]) == 2 {
			Visitor.AddModelTableName(modelName, trim(res[0][1]))
			return
		}
	}
	tableName = strings.Replace(calm2Case(modelName), "model_", "", 1)
	Visitor.AddModelTableName(modelName, tableName)
}

func GenModelCode(packageName, srcPath string) error {
	var KV = struct {
		PackageName string
		FileName    string
		FieldStruct map[string]map[string]ModelFieldStruct
		TableName   map[string]string
		ErrCodeList []*ErrCodeInfo
		IndexMap    map[string]*IndexInfo
		NoScope     bool
		DbType      string
		FreqMap     core.FreqMap
	}{
		PackageName: packageName,
		FileName:    path.Base(srcPath),
		FieldStruct: Visitor.ModelFieldStructMap,
		TableName:   Visitor.ModelTableNameMap,
		ErrCodeList: Visitor.ErrCodeList,
		IndexMap:    Visitor.ModelIndexMap,
		NoScope:     ModelTplNotGenerateGetScopeFunc,
		DbType:      Visitor.dbDriver,
		FreqMap:     Visitor.FreqMap,
	}
	// model 字段 表名 生成
	{
		// 没有表名 不要生成
		if len(KV.TableName) == 0 {
			goto GenModelField
		}
		t, err := template.New("model").Parse(ModelTpl)
		if err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
		t.DefinedTemplates()
		var buf bytes.Buffer
		if err = t.Execute(&buf, KV); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}

		fileDir := path.Dir(srcPath)
		fileName := path.Base(srcPath)
		fileSuffix := path.Ext(fileName)
		filePrefix := fileName[0 : len(fileName)-len(fileSuffix)]

		if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_model_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
	}
GenModelField:
	// model 字段快速获取函数 生成
	{
		// 没有表名 不要生成
		if len(KV.TableName) == 0 {
			goto GenFreqRule
		}
		t, err := template.New("model_field").Parse(ModelFieldTpl)
		if err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
		t.DefinedTemplates()
		var buf bytes.Buffer
		if err = t.Execute(&buf, KV); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}

		fileDir := path.Dir(srcPath)
		fileName := path.Base(srcPath)
		fileSuffix := path.Ext(fileName)
		filePrefix := fileName[0 : len(fileName)-len(fileSuffix)]

		if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_model_field_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
	}
GenFreqRule:
	// 生成限频数据
	{
		if len(Visitor.FreqMap) == 0 {
			goto GenErrCode
		}
		oldPkgName := KV.PackageName

		if FreqRuleOutput != "" {
			FreqRuleOutput = strings.TrimSuffix(FreqRuleOutput, "/")
			// if strings.HasSuffix(FreqRuleOutput, "/") {
			// 	FreqRuleOutput = FreqRuleOutput[:len(FreqRuleOutput)-1]
			// }
			basename := filepath.Base(FreqRuleOutput)
			KV.PackageName = basename
		}

		t, err := template.New("freq_tpl").Parse(FreqTpl)
		if err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
		t.DefinedTemplates()
		var buf bytes.Buffer
		if err = t.Execute(&buf, KV); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}

		// 空输出路径 源目录输出
		if FreqRuleOutput == "" {
			fileDir := path.Dir(srcPath)
			fileName := path.Base(srcPath)
			fileSuffix := path.Ext(fileName)
			filePrefix := fileName[0 : len(fileName)-len(fileSuffix)]

			if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_freq_rule_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
				log.Errorf("err: %+v", err)
				return err
			}
		} else {
			if err := ioutil.WriteFile(fmt.Sprintf("%s/freq_rule.go", FreqRuleOutput), buf.Bytes(), 0666); err != nil {
				log.Errorf("err: %+v", err)
				return err
			}
		}
		KV.PackageName = oldPkgName
	}
GenErrCode:
	// err_code 生成
	{
		// 没有错误代码 不要生成文件
		if len(KV.ErrCodeList) == 0 {
			goto FINALLY
		}
		t, err := template.New("errcode").Parse(ErrCodeTpl)
		if err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
		t.DefinedTemplates()
		var buf bytes.Buffer
		if err = t.Execute(&buf, KV); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}

		fileDir := path.Dir(srcPath)
		fileName := path.Base(srcPath)
		fileSuffix := path.Ext(fileName)
		filePrefix := fileName[0 : len(fileName)-len(fileSuffix)]

		if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_errcode_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
	}

FINALLY:
	return nil
}

// 递归查找父节点是否是 Model
// func isForefatherModelForeachChild(m *proto.Message) bool {
// 	if m == nil {
// 		return false
// 	}

// 	if strings.HasPrefix(m.Name, NameModel) {
// 		return true
// 	}

// 	if tt, ok := m.Parent.(*proto.Message); ok {
// 		return isForefatherModelForeachChild(tt)
// 	}

// 	return false
// }

// getInnerForefathersNameArr 获取 inner 类型 父节点名称数组
func getInnerForefathersNameArr(m *proto.Message) (res []string) {
	if m == nil {
		return
	}

	if strings.HasPrefix(m.Name, NameModel) {
		res = append(res, m.Name)
		return
	}

	if m.Parent == nil {
		return
	}

	if _, ok := m.Parent.(*proto.Message); !ok {
		return
	}

	res = append(res, getInnerForefathersNameArr(m.Parent.(*proto.Message))...)
	res = append(res, m.Name)
	return
}

// getInnerForefathersNameJoin 获取当前节点的全路径
// func getInnerForefathersNameJoin(m *proto.Message) string {
// 	arr := getInnerForefathersNameArr(m)
// 	return strings.Join(arr, "_")
// }

// getInnerForefathersNameDirJoin 获取当前节点的dir路径
// func getInnerForefathersNameDirJoin(m *proto.Message) string {
// 	arr := getInnerForefathersNameArr(m)
// 	arr = arr[:len(arr)-1]
// 	return strings.Join(arr, "_")
// }

// isInnerFirstNode 是否是 inner 类型节点的初始节点
// func isInnerFirstNode(m *proto.Message) bool {
// 	if m == nil {
// 		return false
// 	}

// 	if strings.HasPrefix(m.Name, NameModel) {
// 		return true
// 	}

// 	return false
// }

// isOuterFirstNode 是否是 outer 类型节点的初始节点
// func isOuterFirstNode(m *proto.Message) bool {
// 	if m == nil {
// 		return false
// 	}
// 	if m.Parent == nil {
// 		return false
// 	}

// 	if _, ok := m.Parent.(*proto.Proto); ok {
// 		return true
// 	}

// 	return false
// }

// getOuterForefathersNameArr 获取当前outer类型节点的全路径
func getOuterForefathersNameArr(m *proto.Message) (res []string) {
	if m == nil {
		return
	}

	if m.Parent == nil {
		return
	}

	if _, ok := m.Parent.(*proto.Proto); ok {
		res = append(res, m.Name)
		return
	}

	if msg, ok := m.Parent.(*proto.Message); ok {
		res = append(res, getOuterForefathersNameArr(msg)...)
		res = append(res, m.Name)
	}

	return
}

// getOuterForefathersNameDirJoin outer节点的父节点全路径
// func getOuterForefathersNameDirJoin(m *proto.Message) string {
// 	arr := getOuterForefathersNameArr(m)
// 	arr = arr[:len(arr)-1]
// 	return strings.Join(arr, "_")
// }

// getOuterForefathersNameJoin outer节点全路径
func getOuterForefathersNameJoin(m *proto.Message) string {
	arr := getOuterForefathersNameArr(m)
	return strings.Join(arr, "_")
}

// getModelName 获取父节点的 model name
func getModelName(m *proto.Message) string {
	if m == nil {
		return ""
	}

	if m.Parent == nil {
		return ""
	}

	if _, ok := m.Parent.(*proto.Proto); ok {
		return m.Name
	}

	if msg, ok := m.Parent.(*proto.Message); ok {
		return getModelName(msg)
	}

	return ""
}
