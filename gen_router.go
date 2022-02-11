package proto_parser

import (
	"bytes"
	"fmt"
	"github.com/elliotchance/pie/pie"
	"github.com/emicklei/proto"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"text/template"
)

func parseSrvGenRouter(srv *proto.Service) {
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
		genRouterConfig(srv)
	}
}

func genRouterConfig(srv *proto.Service) {
	var record = new(GroupRouter)
	var apiPrefix string
	var genTo = "internal/controller/impl_controller.go"
	var mws []string
	// 当开启了自定义组前缀
	var doc = srv.Comment.Lines
	for _, com := range doc {
		if strings.Contains(com, "@route_api:") {
			var gra = regexp.MustCompile(RegexpGroupRouterAPI)
			res := gra.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				apiPrefix = trim(res[0][1])
				continue
			}
		}

		if strings.Contains(com, "@gen_to:") {
			var gt = regexp.MustCompile(RegexpRouterGenTo)
			res := gt.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				genTo = trim(res[0][1])
				continue
			}
		}

		// 获取中间件内容部份
		if strings.Contains(com, "@middleware:") {
			mws = prepareMiddleware(com)
		}
	}

	record.RouterPrefix = apiPrefix
	record.GenTo = genTo
	record.Apis = make([]*GroupRouterNode, 0)
	record.Mws = mws

	for _, rpc := range srv.Elements {
		sv := rpc.(*proto.RPC)
		if sv.Comment == nil {
			record.Apis = append(record.Apis, genDefaultGroupRouterNode(sv))
			continue
		}
		if len(sv.Comment.Lines) == 0 {
			record.Apis = append(record.Apis, genDefaultGroupRouterNode(sv))
			continue
		}

		var node = &GroupRouterNode{
			FuncName: sv.Name,
			ReqName:  sv.RequestType,
			RespName: sv.ReturnsType,
			rpc:      sv,
		}
		comment := sv.Comment.Lines
		var isDefaultMethod, isDefaultAPI = true, true
		for _, line := range comment {
			// 检测author
			if strings.Contains(line, "@author:") {
				reg := regexp.MustCompile(RegexpRouterRpcAuthor)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Author = trim(res[0][1])
					continue
				}
			}

			// 检测接口说明
			if strings.Contains(line, "@desc:") {
				reg := regexp.MustCompile(RegexpRouterRpcDesc)
				res := reg.FindAllStringSubmatch(line, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Describe = trim(res[0][1])
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
			// 检测中间件
			if strings.Contains(line, "@middleware") {
				node.Mws = prepareMiddleware(line)

				//reg := regexp.MustCompile(RegexpMiddleware)
				//res := reg.FindAllStringSubmatch(line, -1)
				//if len(res) == 1 && len(res[0]) == 3 {
				//	arr := strings.Split(res[0][2], " ")
				//	for _, s := range arr {
				//		node.Mws = append(node.Mws, s)
				//	}
				//}
			}
		}
		if isDefaultAPI {
			// 默认参数
			node.RouterPath = fmt.Sprintf("/%s", calm2Case(sv.Name))
		}
		if isDefaultMethod {
			// 默认参数
			node.Method = "POST"
		}
		record.Apis = append(record.Apis, node)
	}

	Visitor.AddRouterGroup(srv.Name, record)
}

func genGroupRouterTemplate(pbFile string) error {
	if len(Visitor.GroupRouterMap) == 0 {
		return nil
	}

	var KV = struct {
		PackageName          string
		GroupRouterMap       map[string]*GroupRouter
		GroupRouterImportPkg []string
	}{
		PackageName:          Visitor.PackageName,
		GroupRouterMap:       Visitor.GroupRouterMap,
		GroupRouterImportPkg: pie.Strings(Visitor.GroupRouterImportPkg).Unique(),
	}

	t, err := template.New("group_router").Parse(GroupRouterTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, KV); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	fileDir := path.Dir(pbFile)
	fileName := path.Base(pbFile)
	fileSuffix := path.Ext(fileName)
	filePrefix := strings.Replace(fileName[0:len(fileName)-len(fileSuffix)], "origin_", "", 1)

	if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_router_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	// get proto go_package
	text, _ := GetLineWithchars(pbFile, "option go_package = ")
	var importPackage string
	if text != "" {
		protoGoPkgOption := "option go_package = "
		importPackage = strings.Replace(text, protoGoPkgOption, "", -1)
		importPackage = strings.Replace(importPackage, ";", "", -1)
		importPackage = strings.Replace(importPackage, "\n", "", -1)
		importPackage = strings.Replace(importPackage, "\r", "", -1)
	}

	// 在这里检测文件内容 并注册对应方法实现
	for srvName, router := range KV.GroupRouterMap {
		var at = new(AstTree)
		err := at.parseGoFile(router.GenTo, srvName, importPackage, router.Apis)
		if err != nil {
			logrus.Errorf("parse go file err: %+v", err)
			return err
		}
	}
	return nil
}

func genDefaultGroupRouterNode(rpc *proto.RPC) *GroupRouterNode {
	return &GroupRouterNode{
		FuncName:   rpc.Name,
		RouterPath: fmt.Sprintf("/%s", calm2Case(rpc.Name)),
		Method:     "POST",
		Author:     "@匿名",
		Describe:   "无描述",
		ReqName:    rpc.RequestType,
		RespName:   rpc.ReturnsType,
	}
}
