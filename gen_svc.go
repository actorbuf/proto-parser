package proto_parser

import (
	"regexp"
	"strings"

	"github.com/emicklei/proto"
)

func parseSrvGenRPC(srv *proto.Service) {
	if srv.Comment == nil {
		return
	}

	if len(srv.Comment.Lines) == 0 {
		return
	}

	var needGen bool
	var svcDesc, genTo string
	var doc = srv.Comment.Lines

	for _, com := range doc {
		// 是否需要生成service代码
		if strings.Contains(com, "@rpc_gen") {
			var gr = regexp.MustCompile(RegexpRpcGen)
			res := gr.MatchString(com)
			if res {
				needGen = true
				continue
			}
		}

		// service 注释
		if strings.Contains(com, "@desc") {
			reg := regexp.MustCompile(RegexpRouterRpcDesc)
			res := reg.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				svcDesc = trim(res[0][1])
				continue
			}
		}

		// service 生成位置
		if strings.Contains(com, "@gen_to") {
			var gt = regexp.MustCompile(RegexpRouterGenTo)
			res := gt.FindAllStringSubmatch(com, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				genTo = trim(res[0][1])
				continue
			}
		}
	}

	if !needGen {
		return
	}

	// 开始操作一拨
	genServiceAllRpc(srv, svcDesc, genTo)
}

func genServiceAllRpc(srv *proto.Service, svcDesc, genTo string) {
	// 取 genTo 不存在就创建文件
	// 检测是否实现接口 没实现则生成实现方法
}

func parseSrvGenTask(srv *proto.Service) {
	if srv.Comment == nil {
		return
	}

	if len(srv.Comment.Lines) == 0 {
		return
	}

	lines := srv.Comment.Lines
	isTask := false
	genTo := ""

	for _, doc := range lines {
		if strings.Contains(doc, "@task:") {
			var gt = regexp.MustCompile(RegexpTask)
			if gt.MatchString(doc) {
				isTask = true
				continue
			}
		}
		if strings.Contains(doc, "@gen_to:") {
			var gt = regexp.MustCompile(RegexpRouterGenTo)
			res := gt.FindAllStringSubmatch(doc, -1)
			if len(res) == 1 && len(res[0]) == 2 {
				genTo = trim(res[0][1])
				continue
			}
		}
	}

	if !isTask {
		return
	}

	for _, node := range srv.Elements {
		rpc, ok := node.(*proto.RPC)
		if !ok {
			continue
		}
		if rpc.Comment == nil || len(rpc.Comment.Lines) == 0 {
			continue
		}
		node := TaskNode{}
		docs := rpc.Comment.Lines
		for _, doc := range docs {
			if strings.Contains(doc, "@desc:") {
				reg := regexp.MustCompile(RegexpRouterRpcDesc)
				res := reg.FindAllStringSubmatch(doc, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Desc = trim(res[0][1])
					continue
				}
			}

			if strings.Contains(doc, "@t:") {
				reg := regexp.MustCompile(RegexpTaskTimeSpec)
				res := reg.FindAllStringSubmatch(doc, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Spec = trim(res[0][1])
					continue
				}
			}

			if strings.Contains(doc, "@times:") {
				reg := regexp.MustCompile(RegexpTaskTimes)
				res := reg.FindAllStringSubmatch(doc, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Times = string2Int64(trim(res[0][1]))
					continue
				}
			}

			if strings.Contains(doc, "@range:") {
				reg := regexp.MustCompile(RegexpTaskRange)
				res := reg.FindAllStringSubmatch(doc, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					aa := strings.Split(res[0][1], " ")
					if len(aa) == 2 {
						node.RangeStart = string2Int64(aa[0])
						node.RangeEnd = string2Int64(aa[1])
					}
					continue
				}
			}

			if strings.Contains(doc, "@type:") {
				reg := regexp.MustCompile(RegexpTaskType)
				res := reg.FindAllStringSubmatch(doc, -1)
				if len(res) == 1 && len(res[0]) == 2 {
					node.Type = string2Int64(trim(res[0][1]))
					continue
				}
			}
		}
		Visitor.AddTask(srv.Name, rpc.Name, genTo, node)
	}

	// 代码生成
	TaskCodeGenTo()
}
