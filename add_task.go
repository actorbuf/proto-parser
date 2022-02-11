package proto_parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	"github.com/sirupsen/logrus"
	format "github.com/actorbuf/proto-format"
)

// AddTask 添加一个任务
func AddTask(pbFile, taskSvc, taskName, genTo string) error {
	definition, err := openProtoFile(pbFile)
	if err != nil {
		return err
	}

	proto.Walk(definition,
		proto.WithService(loadServiceList),
	)

	var service *proto.Service
	var exist bool
	// 主任务信息不存在的情况
	_, exist = Visitor.SrvMap[taskSvc]
	if !exist {
		// 开始添加
		service = &proto.Service{
			Comment: &proto.Comment{
				Lines: []string{
					" @task: true",
					fmt.Sprintf(" @gen_to: %s", genTo),
				},
			},
			Name:   taskSvc,
			Parent: definition,
		}

		definition.Elements = append(definition.Elements, service)

		if err := parserFormatWrite(pbFile, definition); err != nil {
			return fmt.Errorf("parse pb file err: %+v", err)
		}
	}

	// 再刷新一次
	definition, err = openProtoFile(pbFile)
	if err != nil {
		return err
	}
	proto.Walk(definition,
		proto.WithService(loadServiceList),
	)
	service, exist = Visitor.SrvMap[taskSvc]
	if !exist {
		return fmt.Errorf("task svc name %s not found", taskSvc)
	}

	// 添加子任务
	// 检查 service 是否有同名子任务
	var canSet = true
	for _, task := range service.Elements {
		rpc := task.(*proto.RPC)
		if rpc.Name == taskName {
			canSet = false
			break
		}
	}

	if !canSet {
		return fmt.Errorf("task name exist")
	}

	service.Elements = append(service.Elements, &proto.RPC{
		Comment: &proto.Comment{
			Lines: []string{
				" @desc: ",
				" 	执行时间规格 分 时 日",
				" @t: 1 * *",
				" 	执行次数",
				" @times: 10",
				" 	执行时间范围 开始秒时间戳 结束秒时间戳",
				" @range: 1640966400 1956499200",
				" 	任务类型 0永续任务 1时间范围执行任务 2指定了执行次数的任务",
				" @type: 0",
			},
		},
		Name:        taskName,
		RequestType: fmt.Sprintf("%sReq", taskName),
		ReturnsType: fmt.Sprintf("%sResp", taskName),
		Parent:      service,
	})

	definition.Elements = append(definition.Elements, &proto.Message{
		Comment: &proto.Comment{},
		Name:    fmt.Sprintf("%sReq", taskName),
		Parent:  definition,
	}, &proto.Message{
		Comment: &proto.Comment{},
		Name:    fmt.Sprintf("%sResp", taskName),
		Parent:  definition,
	})

	if err := parserFormatWrite(pbFile, definition); err != nil {
		return fmt.Errorf("parse pb file err: %+v", err)
	}

	// 格式化一下
	_ = format.Format(pbFile)

	return nil
}

// TaskCodeGenTo 任务代码自动生成
func TaskCodeGenTo() error {
	var KV = struct {
		PackageName string
		TaskConfig  map[string]TaskConfig
	}{
		PackageName: Visitor.PackageName,
		TaskConfig:  Visitor.Tasks,
	}

	t := template.New("task_funcs")
	t.Funcs(template.FuncMap{
		"md5": MD5,
	})
	t, err := t.Parse(GenerateTaskFuncsTemplate)
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

	fileDir := path.Dir(PbFilePath)
	fileName := path.Base(PbFilePath)
	fileSuffix := path.Ext(fileName)
	filePrefix := strings.Replace(fileName[0:len(fileName)-len(fileSuffix)], "origin_", "", 1)

	if err := ioutil.WriteFile(fmt.Sprintf("%s/autogen_task_%s.go", fileDir, filePrefix), buf.Bytes(), 0666); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	// task func generate
	for _, config := range Visitor.Tasks {
		var taskKV = struct {
			PackageName string
			TaskNode    map[string]TaskNode
		}{
			PackageName: Visitor.PackageName,
			TaskNode:    config.Task,
		}

		t, err := template.New("task_func").Parse(GenerateTaskFuncTemplate)
		if err != nil {
			logrus.Errorf("err: %+v", err)
			return err
		}

		t.DefinedTemplates()
		var buf bytes.Buffer
		if err = t.Execute(&buf, taskKV); err != nil {
			logrus.Errorf("err: %+v", err)
			return err
		}

		fileName := path.Base(config.GenTo)
		if !strings.HasSuffix(fileName, ".go") {
			fileName = fmt.Sprintf("%s.go", fileName)
		}
		if err := ioutil.WriteFile(fmt.Sprintf("%s/%s", fileDir, fileName), buf.Bytes(), 0666); err != nil {
			logrus.Errorf("err: %+v", err)
			return err
		}
	}

	return nil
}
