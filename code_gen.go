package proto_parser

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/elliotchance/pie/pie"
	"github.com/emicklei/proto"
	log "github.com/sirupsen/logrus"
	format "github.com/actorbuf/proto-format"
	"github.com/actorbuf/proto-parser/rename"
)

// listProtoFile 获取当前目录下的所有 proto 文件
func listProtoFile(path string) []string {
	path = strings.TrimSuffix(path, "/")
	// if strings.HasSuffix(path, "/") {
	// 	path = path[:len(path)-1]
	// }
	var result []string
	fi, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err)
	}
	for _, file := range fi {
		if file.IsDir() {
			result = append(result, listProtoFile(path+"/"+file.Name())...)
		}
		if strings.HasPrefix(file.Name(), "autogen") || strings.HasPrefix(file.Name(), "origin") {
			_, _ = fmt.Fprintf(os.Stdout, "prefix: autogen, rule ignore %s!\n", path+"/"+file.Name())
			continue
		}
		if strings.HasSuffix(file.Name(), ".proto") {
			result = append(result, path+"/"+file.Name())
		}
	}
	return result
}

// execGoFmt 执行一次 gofmt 操作 做到风格统一
func execGoFmt(genTo string) {
	if genTo == "" {
		genTo = "."
	}
	_, err := exec.Command("gofmt", "-w", genTo).CombinedOutput()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "gofmt %s err: %+v\n", genTo, err)
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, "gofmt %s successful!\n", genTo)
}

// CodeGen 代码自动生成
// 自动注入bson; 自动抽离 model 的字段
// 自定义注入json; 自定义表名 table_name
// 自动生成路由->请求结构体映射; 自动生成mongo index
func CodeGen(config *CodeGenConfig) error {
	if config == nil {
		return fmt.Errorf("配置项为空")
	}

	// 预先需要处理的全局变量
	ModelTplNotGenerateGetScopeFunc = config.NoGetScopeFunc
	FreqRuleOutput = config.FreqOutput

	var pbFileList []string
	fi, err := os.Stat(config.PbFilePath)
	if err != nil {
		log.Errorf("CodeGen err: %+v", err)
		return err
	}

	if fi.IsDir() {
		// 是目录 需要遍历目录 扫描一下 .proto结尾的文件
		pbFileList = listProtoFile(config.PbFilePath)
	} else if strings.HasSuffix(config.PbFilePath, "*.proto") || strings.HasSuffix(config.PbFilePath, "*") {
		pbFileList = listProtoFile(path.Dir(config.PbFilePath))
	}

	// 取目录 取文件名
	dirName := path.Dir(config.PbFilePath)
	baseName := path.Base(config.PbFilePath)

	if strings.HasPrefix(config.PbFilePath, "autogen") || strings.HasPrefix(config.PbFilePath, "origin") {
		_, _ = fmt.Fprintf(os.Stdout, "prefix: autogen, ignore %s!\n", config.PbFilePath)
	} else if strings.HasSuffix(baseName, ".proto") {
		pbFileList = append(pbFileList, dirName+"/"+baseName)
	}

	defer func() {
		_, _ = fmt.Fprintf(os.Stdout, "generate list: %+v\n", pbFileList)
	}()

	pbFileList = pie.Strings(pbFileList).Unique()

	if len(pbFileList) == 0 {
		return nil
	}

	for _, pbPath := range pbFileList {
		Visitor = &ProtoVisitor{dbDriver: config.DbDriveType}
		// 格式化后 转移源文件
		oriDir := path.Dir(pbPath)
		oriName := path.Base(pbPath)
		midName := fmt.Sprintf("%s/origin_%s", oriDir, oriName)
		if err := renameOriginFile(pbPath, midName); err != nil {
			log.Errorf("err: %+v", err)
			return err
		}

		var args []string

		injPath, err := ParseProto(midName)
		if err != nil {
			log.Errorf("err: %+v", err)
			return err
		}
		// 格式化工作放在 parse proto 之后
		_ = format.Format(midName)

		if config.OutputPath == "" {
			config.OutputPath = path.Dir(injPath)
		}
		goOut := fmt.Sprintf("--go_out=%s", config.OutputPath)
		args = append(args, goOut)

		// grpc 开启
		if config.GrpcOutputPath != "" {
			grpcOut := fmt.Sprintf("--go-grpc_out=%s", config.GrpcOutputPath)
			args = append(args, grpcOut)
		}

		args = append(args, injPath)

		var incs []string
		for _, s := range config.IncludePbFiles {
			incs = append(incs, fmt.Sprintf("-I%s", s))
		}

		args = append(args, incs...)

		protocCmd := exec.Command("protoc", args...)
		_, _ = fmt.Fprintf(os.Stdout, "run cmd: %s\n", protocCmd.String())
		protocOut, err := protocCmd.CombinedOutput()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "run err: %+v\nrun out: %+v\n", err, string(protocOut))
			_ = removeInjectBsonFile(injPath)
			return err
		}

		baseName := path.Base(injPath)
		filePrefix := baseName[0 : len(baseName)-len(path.Ext(injPath))]
		pbCodeFile := fmt.Sprintf("%s/%s.pb.go", path.Dir(injPath), filePrefix)

		target := fmt.Sprintf("-input=%s", pbCodeFile)
		args = []string{target}
		injTagCmd := exec.Command("protoc-go-inject-tag", args...)
		_, _ = fmt.Fprintf(os.Stdout, "run cmd: %s\n", injTagCmd.String())
		injTagOut, err := injTagCmd.CombinedOutput()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "run err: %v\nrun out: %v\n", err, string(injTagOut))
			_ = removeInjectBsonFile(injPath)
			return err
		}

		_ = removeInjectBsonFile(injPath)
		_, _ = fmt.Fprintf(os.Stdout, "%s code generate complete!\n", pbPath)
	}

	// 格式化一次 输出路径
	if config.OutputNeedFormat {
		execGoFmt(config.OutputPath)
	}

	return nil
}

func removeInjectBsonFile(src string) error {
	// 将中间文件删除
	if err := os.Remove(src); err != nil {
		return err
	}
	// 将源文件名复原
	dir := path.Dir(src)
	base := path.Base(src)
	oldFile := fmt.Sprintf("%s/origin_%s", dir, base)

	return rename.Atomic(oldFile, src)
}

func renameOriginFile(src string, dst string) error {
	return rename.Atomic(src, dst)
}

// AddAPI 生成一个API 并自动生成Req/Resp
func AddAPI(pbFile, groupRouter, apiName, method string) error {
	reader, err := os.Open(pbFile)
	if err != nil {
		log.Errorf("open pb file: %+v, get err: %+v", pbFile, err)
		return err
	}
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, err := parser.Parse()
	if err != nil {
		log.Errorf("parse pb file err: %+v", err)
		return err
	}

	proto.Walk(definition, proto.WithService(addAPI))

	var srv *proto.Service
	var existSrv bool
	if srv, existSrv = Visitor.APIGroupSrvMap[groupRouter]; !existSrv {
		return fmt.Errorf("group router name: %s not found", groupRouter)
	}

	rpcsList := srv.Elements
	for _, rpc := range rpcsList {
		v, ok := rpc.(*proto.RPC)
		if ok && v.Name == apiName {
			return fmt.Errorf("api name is exist!")
		}
	}

	if method == "" {
		method = "POST"
	}
	// 注入 rpc
	srv.Elements = append(srv.Elements, &proto.RPC{
		Comment: &proto.Comment{
			Lines: []string{
				" @desc: ",
				" @author: ",
				fmt.Sprintf(" @method: %s", method),
				fmt.Sprintf(" @api: /%s", calm2Case(apiName)),
				" @middleware: ",
			},
		},
		Name:        apiName,
		RequestType: fmt.Sprintf("%sReq", apiName),
		ReturnsType: fmt.Sprintf("%sResp", apiName),
		Parent:      srv,
	})

	// 注入req resp
	definition.Elements = append(definition.Elements, &proto.Message{
		Name:   fmt.Sprintf("%sReq", apiName),
		Parent: definition,
	})
	definition.Elements = append(definition.Elements, &proto.Message{
		Name:   fmt.Sprintf("%sResp", apiName),
		Parent: definition,
	})

	if err := parserFormatWrite(pbFile, definition); err != nil {
		log.Errorf("write pb file err: %+v", err)
		return err
	}

	// 格式化一拨
	_ = format.Format(pbFile)

	return nil
}

func addAPI(srv *proto.Service) {
	var needGen bool
	if strings.HasSuffix(srv.Name, NameAPIGroup) {
		needGen = true
	}
	if !needGen {
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
			needGen = true
		}
	}

	if needGen {
		Visitor.AddApiSrv(srv.Name, srv)
	}
}

// AddRPC 生成一个RPC 并自动生成Req/Resp
func AddRPC(pbFile, srvName, apiName string) error {
	reader, err := os.Open(pbFile)
	if err != nil {
		log.Errorf("open pb file: %+v, get err: %+v", pbFile, err)
		return err
	}
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, err := parser.Parse()
	if err != nil {
		log.Errorf("parse pb file err: %+v", err)
		return err
	}

	proto.Walk(definition, proto.WithService(addRPC))

	var srv *proto.Service
	var existSrv bool
	if srv, existSrv = Visitor.SrvMap[srvName]; !existSrv {
		return fmt.Errorf("service name: %s not found", srvName)
	}

	// 注入 rpc
	srv.Elements = append(srv.Elements, &proto.RPC{
		Comment: &proto.Comment{
			Lines: []string{
				" @desc:",
				" @author:",
			},
		},
		Name:        apiName,
		RequestType: fmt.Sprintf("%sReq", apiName),
		ReturnsType: fmt.Sprintf("%sResp", apiName),
		Parent:      srv,
	})

	// 注入req resp
	definition.Elements = append(definition.Elements, &proto.Message{
		Name:   fmt.Sprintf("%sReq", apiName),
		Parent: definition,
	})
	definition.Elements = append(definition.Elements, &proto.Message{
		Name:   fmt.Sprintf("%sResp", apiName),
		Parent: definition,
	})

	if err := parserFormatWrite(pbFile, definition); err != nil {
		log.Errorf("write pb file err: %+v", err)
		return err
	}

	// 格式化一拨
	_ = format.Format(pbFile)

	return nil
}

func addRPC(srv *proto.Service) {
	Visitor.AddSrv(srv.Name, srv)
}
