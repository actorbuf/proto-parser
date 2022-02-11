package proto_parser

import (
	"fmt"
	"go/ast"
)

// ecInfo 是 errorCode info 的简称
type ecInfo struct {
	pkg      string
	name     string
	fullName string
}

// getFuncDeclName 获取 FuncDecl 函数名称
func getFuncDeclName(f *ast.FuncDecl) string {
	return f.Name.Name
}

// getFuncDeclBody 获取 FuncDecl body
// func getFuncDeclBody(f *ast.FuncDecl) *ast.BlockStmt {
// 	return f.Body
// }

// iterBodyToGetCreateErrorStmt 获取 CreateError
func iterBodyToGetCreateErrorStmt(body interface{}) (bool, []ecInfo) {
	var res []ecInfo
	switch t := body.(type) {
	case *ast.BlockStmt:
		// 是 body 取行 再递归
		for _, stmt := range t.List {
			ok, r := iterBodyToGetCreateErrorStmt(stmt)
			if ok {
				res = append(res, r...)
			}
		}
	case *ast.AssignStmt:
		// 是申明
		for _, rh := range t.Rhs {
			ok, r := iterBodyToGetCreateErrorStmt(rh)
			if ok {
				res = append(res, r...)
			}
		}
	case *ast.CallExpr:
		fun, ok := t.Fun.(*ast.SelectorExpr)
		if ok {
			x, xok := fun.X.(*ast.Ident)
			if !xok {
				return false, nil
			}
			if x.Name != "core" {
				return false, nil
			}
			if fun.Sel.Name != "CreateError" && fun.Sel.Name != "CreateErrorWithMsg" {
				return false, nil
			}
			if len(t.Args) < 1 {
				return false, nil
			}
			val, aok := t.Args[0].(*ast.SelectorExpr)
			if !aok {
				return false, nil
			}
			pkg, pok := val.X.(*ast.Ident)
			if !pok {
				return false, nil
			}
			code := val.Sel.Name
			res = append(res, ecInfo{
				pkg:      pkg.Name,
				name:     code,
				fullName: fmt.Sprintf("%s.%s", pkg.Name, code),
			})
			return true, res
		}
	case *ast.ReturnStmt:
		for _, result := range t.Results {
			ok, r := iterBodyToGetCreateErrorStmt(result)
			if ok {
				res = append(res, r...)
			}
		}
	case *ast.IfStmt:
		ok, r := iterBodyToGetCreateErrorStmt(t.Body)
		if ok {
			res = append(res, r...)
		}
	case *ast.ForStmt:
		ok, r := iterBodyToGetCreateErrorStmt(t.Body)
		if ok {
			res = append(res, r...)
		}
	case *ast.SwitchStmt:
		ok, r := iterBodyToGetCreateErrorStmt(t.Body)
		if ok {
			res = append(res, r...)
		}
	case *ast.SelectStmt:
		ok, r := iterBodyToGetCreateErrorStmt(t.Body)
		if ok {
			res = append(res, r...)
		}
	}
	if len(res) != 0 {
		return true, res
	}
	return false, nil
}
