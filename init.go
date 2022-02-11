package proto_parser

import (
	"fmt"
	nested "github.com/antonfisher/nested-logrus-formatter"
	log "github.com/sirupsen/logrus"
	"os"
	"runtime"
	"strings"
)

func init() {
	var getServeDir = func(path string) string {
		var run, _ = os.Getwd()
		return strings.Replace(path, run, ".", -1)
	}
	var formatter = &nested.Formatter{
		NoColors:        false,
		HideKeys:        true,
		TimestampFormat: "2006-01-02 15:04:05",
		CallerFirst:     true,
		CustomCallerFormatter: func(f *runtime.Frame) string {
			s := strings.Split(f.Function, ".")
			funcName := s[len(s)-1]
			return fmt.Sprintf(" [%s:%d][%s()]", getServeDir(f.File), f.Line, funcName)
		},
	}
	if runtime.GOOS == "windows" {
		formatter.NoColors = true
	}
	log.SetFormatter(formatter)
	log.SetReportCaller(true)
}
