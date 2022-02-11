package proto_parser

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	format "github.com/actorbuf/proto-format"
)

func TestCodeGen(t *testing.T) {
	err := CodeGen(&CodeGenConfig{
		PbFilePath:       "./proto/code.proto",
		OutputPath:       "./proto",
		GrpcOutputPath:   "",
		IncludePbFiles:   nil,
		OutputNeedFormat: false,
		NoGetScopeFunc:   true,
	})
	if err != nil {
		panic(err)
	}
}

func TestGormCodeGen(t *testing.T) {
	err := CodeGen(&CodeGenConfig{
		PbFilePath:     "./parser.proto",
		OutputPath:     ".",
		NoGetScopeFunc: false,
		DbDriveType:    "gdbc",
	})
	if err != nil {
		logrus.Errorf("get err: %+v", err)
	}
}

func TestAddApi(t *testing.T) {
	err := AddAPI("./parser.proto", "TestService", "Find", "POST")
	if err != nil {
		panic(err)
	}
}

func TestAddRPC(t *testing.T) {
	err := AddRPC("router_group.proto", "MemberInfo", "Find")
	if err != nil {
		panic(err)
	}
}

func TestOutputMD(t *testing.T) {
	err := OutputMD("./parser.proto", "CSecRisk", "AutoCheck", []string{})
	if err != nil {
		panic(err)
	}
}

func TestParseGoFile(t *testing.T) {
	var ag = new(AstTree)
	err := ag.parseGoFile("./freq_controller.go", "Freq", "", nil)
	if err != nil {
		panic(err)
	}

	//ag.checkAndGenerateRouteStruct()
}

func TestFsDir(t *testing.T) {
	name := path.Base(path.Dir("./gen_to.go"))
	fmt.Println(name)

	if name == "." {
		realPath, _ := os.Getwd()
		fmt.Println(realPath)

		fmt.Println(filepath.Base(realPath))
	}
}

func TestFormat(t *testing.T) {
	fmt.Println(format.Format("./model/parser.proto"))
}

func TestAddRoute(t *testing.T) {
	err := AddRoute("./parser.proto", "TestService", "/api/test", "./internal/controller/test_controller.go")
	if err != nil {
		fmt.Println(err)
	}
}

func TestAddSVC(t *testing.T) {
	err := AddSvc("./parser.proto", "TestService", "./internal/services/test_service.go")
	if err != nil {
		fmt.Println(err)
	}
}

func TestAddTask(t *testing.T) {
	err := AddTask("./proto/code.proto", "TestService", "GetTwo", "./internal/services/test_service.go")
	if err != nil {
		panic(err)
	}
}
