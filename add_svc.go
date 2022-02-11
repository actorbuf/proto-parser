package proto_parser

import (
	"fmt"
	"github.com/emicklei/proto"
)

// AddSvc 生成service
func AddSvc(pbFile, svcName, genTo string) error {
	definition, err := openProtoFile(pbFile)
	if err != nil {
		return err
	}

	proto.Walk(definition,
		proto.WithService(loadServiceList),
	)

	if _, exist := Visitor.SrvMap[svcName]; exist {
		return fmt.Errorf("%s has been exist", svcName)
	}

	// 开始添加
	var service = &proto.Service{
		Comment: &proto.Comment{
			Lines: []string{
				" @desc: ",
				" @rpc_gen: true",
				fmt.Sprintf(" @gen_to: %s", genTo),
			},
		},
		Name:     svcName,
		Elements: nil,
		Parent:   definition,
	}

	definition.Elements = append(definition.Elements, service)

	if err := parserFormatWrite(pbFile, definition); err != nil {
		return fmt.Errorf("parse pb file err: %+v", err)
	}
	return nil
}
