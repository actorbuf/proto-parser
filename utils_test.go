package proto_parser

import (
	"fmt"
	"strings"
	"testing"
)

func Test_calm2KebabCase(t *testing.T) {
	src := calm2KebabCaseBSON("HelloWorld")
	t.Log(src)
}

func TestTrim(t *testing.T) {
	var a = " dddd\n"
	a = strings.Trim(a, " \n")
	fmt.Println(a)
}
