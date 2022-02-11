package proto_parser

import (
	"fmt"

	"github.com/emicklei/proto"
)

func loadServiceList(srv *proto.Service) {
	Visitor.AddSrv(srv.Name, srv)
}

// AddRoute 生成路由组
func AddRoute(pbFile, routeName, api, genTo string) error {
	definition, err := openProtoFile(pbFile)
	if err != nil {
		return err
	}

	proto.Walk(definition,
		proto.WithService(loadServiceList),
	)

	if _, exist := Visitor.SrvMap[routeName]; exist {
		return fmt.Errorf("%s has been exist", routeName)
	}

	// 开始添加
	var service = &proto.Service{
		Comment: &proto.Comment{
			Lines: []string{
				" @route_group: true",
				fmt.Sprintf(" @route_api: %s", api),
				fmt.Sprintf(" @gen_to: %s", genTo),
				" @middleware: ",
			},
		},
		Name:     routeName,
		Elements: nil,
		Parent:   definition,
	}

	definition.Elements = append(definition.Elements, service)

	if err := parserFormatWrite(pbFile, definition); err != nil {
		return fmt.Errorf("parse pb file err: %+v", err)
	}
	return nil
}
