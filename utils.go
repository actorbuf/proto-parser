package proto_parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/emicklei/proto"
	"github.com/emicklei/proto-contrib/pkg/protofmt"
)

func string2Int64(s string) int64 {
	b, _ := strconv.ParseInt(s, 10, 64)
	return b
}

// 首字母大写
func toTitle(src string) string {
	var upperStr string
	vv := []rune(src)
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {
				vv[i] -= 32 // string的码表相差32位
				upperStr += string(vv[i])
			} else {
				return src
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}

// calm2Case 驼峰转下划线
func calm2Case(src string) string {
	buffer := new(bytes.Buffer)
	for i, r := range src {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteRune('_')
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}
	return buffer.String()
}

// calm2KebabCase 驼峰转短横线
// func calm2KebabCase(src string) string {
// 	buffer := new(bytes.Buffer)
// 	for i, r := range src {
// 		if unicode.IsUpper(r) {
// 			if i != 0 {
// 				buffer.WriteRune('-')
// 			}
// 			buffer.WriteRune(unicode.ToLower(r))
// 		} else {
// 			buffer.WriteRune(r)
// 		}
// 	}
// 	return buffer.String()
// }

// calm2KebabCaseBSON 驼峰转短横线
func calm2KebabCaseBSON(src string) string {
	if src == "Id" || src == "ID" {
		return "_id"
	}
	if strings.Contains(src, "ID") {
		src = strings.ReplaceAll(src, "ID", "Id")
	}
	buffer := new(bytes.Buffer)
	for i, r := range src {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteRune('-')
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}
	return buffer.String()
}

// GormTagGenerate gorm注入tag
func GormTagGenerate(field, tag, typ string) string {
	var res strings.Builder
	docSplit := strings.Split(tag, ";")

	var getType = func(t string) string {
		switch t {
		case "string":
			return "type:string;"
		case "float", "double":
			return "type:float;"
		case "bool":
			return "type:bool;"
		case "uint32", "uint64", "fixed32", "fixed64":
			return "type:uint;"
		case "int32", "int64", "sint32", "sint64", "sfixed32", "sfixed64":
			return "type:int;"
		case "bytes":
			return "type:bytes;"
		}
		return ""
	}

	// 没有声明 gorm 的标记的时候 注入 column
	if len(docSplit) <= 1 {
		if field == "Id" || field == "ID" || field == "id" {
			res.WriteString("primaryKey;autoIncrement;")
		}
		// 注入字段名
		column := calm2Case(field)
		res.WriteString("column:" + column + ";")
		// 注入默认的字段类型
		switch typ {
		case "string":
			res.WriteString("type:string;")
		case "float", "double":
			res.WriteString("type:float;")
		case "bool":
			res.WriteString("type:bool;")
		case "uint32", "uint64", "fixed32", "fixed64":
			res.WriteString("type:uint;")
		case "int32", "int64", "sint32", "sint64", "sfixed32", "sfixed64":
			res.WriteString("type:int;")
		case "bytes":
			res.WriteString("type:bytes;")
		}
		return res.String()
	}

	var docMap = make(map[string]string)
	for _, tt := range docSplit {
		// 再拆分一下
		if strings.Contains(tt, ":") {
			kv := strings.Split(tt, ":")
			docMap[kv[0]] = kv[1]
			continue
		}
		docMap[tt] = ""
	}

	// 检测是否配置了 column
	if _, exist := docMap["column"]; !exist {
		docMap["column"] = GormTagGenerate(field, "", typ)
	}

	// 检测是否配置了 type
	if _, exist := docMap["type"]; !exist {
		docMap["type"] = getType(typ)
	}

	return ""
}

// calm2CaseBSON 驼峰转bson下划线
func calm2CaseBSON(src string) string {
	if src == "Id" || src == "ID" {
		return "_id"
	}
	if strings.Contains(src, "ID") {
		src = strings.ReplaceAll(src, "ID", "Id")
	}
	buffer := new(bytes.Buffer)
	for i, r := range src {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteRune('_')
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}
	return buffer.String()
}

// case2Camel 下划线转大驼峰
func case2Camel(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

// case2LowerCamel 下划线转小驼峰
func case2LowerCamel(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	name = strings.Replace(name, " ", "", -1)
	first := strings.ToLower(string(name[0]))
	return first + name[1:]
}

// parserFormatWrite 写入格式化proto
func parserFormatWrite(filename string, parserProto *proto.Proto) error {
	if parserProto == nil {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		parser := proto.NewParser(file)
		parserProto, err = parser.Parse()
		if err != nil {
			return err
		}
	}

	var buf = new(bytes.Buffer)

	protofmt.NewFormatter(buf, "    ").Format(parserProto) // 1 tab

	// write back to input
	if err := os.WriteFile(filename, buf.Bytes(), os.ModePerm); err != nil {
		return err
	}

	return nil
}

// isBuiltInType 是否是内置类型
func isBuiltInType(typ string) bool {
	switch typ {
	case "string", "uint32", "uint64", "int32", "int64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64", "bool", "bytes", "float", "double":
		return true
	}
	return false
}

// fixPkgName 修正不规范的包名
func fixPkgName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}

// trim 替换windows的换行效果 替换左右的空格
func trim(src string) string {
	if runtime.GOOS == "windows" {
		src = strings.ReplaceAll(src, "\r\n", "")
		src = strings.ReplaceAll(src, "\n\t", "")
	}
	src = strings.ReplaceAll(src, "\n", "")
	src = strings.ReplaceAll(src, "\t", "")
	src = strings.ReplaceAll(src, "\r", "")
	src = strings.Trim(src, " ")
	return src
}

// getErrorCodeField 基于 *proto.Enum 获取某个字段
func getErrorCodeField(e *proto.Enum) {
	for _, element := range e.Elements {
		ef, ok := element.(*proto.EnumField)
		if !ok {
			continue
		}
		if Visitor.MDocDepPkgName == "" {
			continue
		}
		var v = MDocsErrCodeField{Code: ef.Integer, Pkg: Visitor.MDocDepPkgName}
		if ef.InlineComment != nil {
			v.Desc = trim(ef.InlineComment.Message())
		}
		Visitor.AddErrCodeEnumField(fmt.Sprintf("%s.%s", Visitor.MDocDepPkgName, ef.Name), v)
	}
}

// getRpcErrCodeMap 获取rpc的错误码map
func getRpcErrCodeMap(doc []string) {
	var segStart = -1
	for i, s := range doc {
		if !strings.Contains(s, "@error") {
			continue
		}
		segStart = i
		break
	}
	// 没有错误码
	if segStart == -1 {
		return
	}
	var segEnd = segStart
	for i := segStart; i < len(doc); i++ {
		if strings.Contains(doc[i], "@error") {
			continue
		} else if strings.Contains(doc[i], "@") {
			break
		}
		segEnd = i
	}
	// 没有错误码 只有 @error标签
	if segStart == segEnd {
		return
	}
	var errMap = make(map[string]MDocsErrCodeField)
	for i := segStart; i <= segEnd; i++ {
		// 包含了 @ 标记的 过滤掉
		if strings.Contains(doc[i], "@") {
			continue
		}
		errMap[trim(doc[i])] = MDocsErrCodeField{}
	}
	if Visitor.MDoc == nil {
		Visitor.MDoc = new(MDocs)
	}
	Visitor.MDoc.ErrCodeMap = errMap
}

// getInlineComment 获取行内注释
func getInlineComment(field *proto.NormalField) string {
	if field == nil || field.InlineComment == nil {
		return ""
	}
	return strings.Trim(field.InlineComment.Message(), " \n")
}

// prepareMiddleware 准备中间件处理
func prepareMiddleware(doc string) []string {
	var mw = regexp.MustCompile(RegexpMiddlewareContent)
	res := mw.FindAllStringSubmatch(doc, -1)
	if len(res) != 1 {
		return nil
	}
	if len(res[0]) != 2 {
		return nil
	}
	content := res[0][1]
	var reg2 = regexp.MustCompile(RegexpMiddlewareFunc)
	dd := reg2.FindAllStringSubmatch(content, -1)

	if len(dd) == 0 {
		return nil
	}

	var upkgs []string

	for _, v := range dd {
		if len(v) != 3 {
			continue
		}
		Visitor.GroupRouterImportPkg = append(Visitor.GroupRouterImportPkg, v[1])
		pkg := filepath.Base(v[1])
		mws := strings.Split(v[2], ",")
		for _, mw := range mws {
			mw = strings.TrimSpace(mw)
			upkgs = append(upkgs, fmt.Sprintf("%s.%s", pkg, mw))
		}
	}

	// 导入中间件所需要的依赖包
	if len(upkgs) != 0 {
		Visitor.GroupRouterImportPkg = append(Visitor.GroupRouterImportPkg, "github.com/gin-gonic/gin")
	}

	return upkgs
}

// getSvcRouterAPI 从service中获取路由前缀
func getSvcRouterAPI(doc []string) string {
	for _, com := range doc {
		if strings.Contains(com, "@route_api:") {
			var gra = regexp.MustCompile(RegexpGroupRouterAPI)
			res := gra.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				return trim(res[0][1])
			}
		}
	}
	return ""
}

// getRpcRouterAPI 从rpc中获取路由后缀
func getRpcRouterAPI(doc []string) string {
	for _, com := range doc {
		if strings.Contains(com, "@api") {
			reg := regexp.MustCompile(RegexpRouterRpcURL)
			res := reg.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				suffix := res[0][1]
				if !strings.HasPrefix(suffix, "/") {
					suffix = fmt.Sprintf("/%s", suffix)
				}
				return suffix
			}
		}
	}
	return ""
}

func IsExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}

func GetLineWithchars(filePath, chars string) (lineText string, err error)  {
	lineText = ""
	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		err = errors.New(filePath + " open file error: " + err.Error())
		return
	}
	//建立缓冲区，把文件内容放到缓冲区中
	buf := bufio.NewReader(f)
	for {
		//遇到\n结束读取
		var b []byte
		b, err = buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}
		text := string(b)
		if strings.Contains(text, chars) {
			lineText = text
		}
	}

	return
}

func GetFileLineOne(filePath string) (lineOneText string, err error) {
	lineOneText = ""
	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		err = errors.New(filePath + " open file error: " + err.Error())
		return
	}
	//建立缓冲区，把文件内容放到缓冲区中
	buf := bufio.NewReader(f)
	//遇到\n结束读取
	b, err := buf.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			err = errors.New(filePath + " is empty! ")
			return
		}
		err = errors.New(filePath + " read bytes error: " + err.Error())
		return
	}
	lineOneText = string(b)
	err = nil
	return
}

func GetCurrentModuleName() (modName string, err error) {
	modName = ""

	text, err := GetFileLineOne("go.mod")
	if err != nil {
		return
	}

	if len(text) < 8 {
		err = errors.New("go.mod文件格式不正确！")
		return
	}

	modName = text[7 : len(text)-1]
	modName = strings.Trim(modName, "\r")
	modName = strings.Trim(modName, "\n")

	return
}