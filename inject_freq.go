package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
	"github.com/actorbuf/iota/core"
	"regexp"
	"strings"
)

func injectFreqMap(svc *proto.Service) {
	if len(svc.Elements) == 0 {
		return
	}
	if svc.Comment == nil || len(svc.Comment.Lines) == 0 {
		return
	}
	// 取路由前缀
	prefix := getSvcRouterAPI(svc.Comment.Lines)

	for _, element := range svc.Elements {
		rpc, ok := element.(*proto.RPC)
		if !ok {
			continue
		}
		if rpc.Comment == nil || len(rpc.Comment.Lines) == 0 {
			continue
		}
		doc := rpc.Comment.Lines
		suffix := getRpcRouterAPI(doc)
		for _, c := range doc {
			if strings.Contains(c, "@freq") {
				r := regexp.MustCompile(RegexpFreq)
				res := r.FindAllStringSubmatch(c, -1)
				if len(res) != 1 {
					fmt.Println("限频格式错误: ", c)
					continue
				}
				if len(res[0]) != 2 {
					fmt.Println("限频格式错误: ", c)
					continue
				}
				freqData := trim(res[0][1])
				freqS := strings.Split(freqData, " ")
				if len(freqS) != 3 {
					fmt.Println("限频格式错误: ", freqS)
					continue
				}
				c := core.FreqConfig{
					Minute: string2Int64(freqS[0]),
					Hour:   string2Int64(freqS[1]),
					Day:    string2Int64(freqS[2]),
				}
				Visitor.AddFreq(fmt.Sprintf("%s%s", prefix, suffix), c)
			}
		}
	}
}
