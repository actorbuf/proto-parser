package proto_parser

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/elliotchance/pie/pie"
	"github.com/emicklei/proto"
	"github.com/sirupsen/logrus"
)

// 用来解析.go文件 识别是否实现路由组 GRPC

type AstTree struct {
	pos              *token.FileSet
	fileTree         *ast.File
	goPath           string
	srvName          string
	pkgName          string
	groupRouterNode  []*GroupRouterNode
	needCreate       bool
	needImplBindFunc bool // 是否需要实现 bind 方法
	importPackage    string
}

// parseGoFile 解析文件
func (tree *AstTree) parseGoFile(goFile string, srvName string, importPackage string, routers []*GroupRouterNode) error {
	file, err := ioutil.ReadFile(goFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree.needCreate = true
		} else {
			logrus.Errorf("read file err: %+v", err)
			return err
		}
	}

	//logrus.Infof("srv name: %+v", srvName)
	tree.goPath = goFile
	tree.srvName = srvName
	tree.pkgName = path.Base(path.Dir(goFile))
	tree.groupRouterNode = routers
	tree.needImplBindFunc = true
	tree.importPackage = importPackage

	if tree.pkgName == "." {
		realPath, _ := os.Getwd()
		tree.pkgName = fixPkgName(filepath.Base(realPath))
	}

	if tree.needCreate {
		// 直接创建文件
		_ = tree.generateFile()
		return nil
	}

	// Create the AST by parsing src.
	tree.pos = token.NewFileSet()
	tree.fileTree, err = parser.ParseFile(tree.pos, goFile, file, parser.ParseComments)
	if err != nil {
		logrus.Errorf("parse file err: %+v", err)
		return err
	}

	tree.checkAndGenerateRouteStruct()

	// Print the AST.
	//err = ast.Print(tree.pos, tree.fileTree)
	//if err != nil {
	//	logrus.Errorf("print file err: %+v", err)
	//	return err
	//}

	return nil
}

// checkAndGenerateRouteStruct 检查路由struct 是否全部实现 没有完全实现 则补全 完全没实现 则生成 完全实现了 则不操作
func (tree *AstTree) checkAndGenerateRouteStruct() {
	// 检查 srvName 是否存在
	var existSrvName bool
	if tree.fileTree == nil {
		return
	}
GetResult:
	for _, decl := range tree.fileTree.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if ts.Name.Name == tree.srvName {
				existSrvName = true
				break GetResult
			}
		}
	}

	if !existSrvName {
		_ = tree.generateAll()
		return
	}

	// 检查 srvName 对应的方法是否全部实现
	var noImplNode []*GroupRouterNode
Restart:
	for _, node := range tree.groupRouterNode {
		for _, decl := range tree.fileTree.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if fd.Name != nil {
				if fd.Name.Name == node.FuncName {
					// 取注释
					//if fd.Doc != nil {
					//	logrus.Infof("value: %+v", fd.Doc.Text())
					//	split := strings.Split(trim(fd.Doc.Text()), " ")
					//	logrus.Infof("len: %+v, %+v", len(split), split)
					//}
					Visitor.AddImplRouter(tree.srvName, tree.goPath, node.rpc)
					continue Restart
				}
				// Bind 是否实现了
				if fd.Name.Name == "Bind" {
					tree.needImplBindFunc = false
				}
			}

			//if fd.Doc != nil {
			//	logrus.Infof("name: %+v, value: %+v", fd.Name.Name, fd.Doc.Text())
			//}
		}
		noImplNode = append(noImplNode, node)
	}

	if tree.needImplBindFunc {
		_ = tree.generateBindFunc()
	}

	if len(noImplNode) == 0 {
		return
	}

	tree.groupRouterNode = noImplNode

	_ = tree.generateFunc()

	// 全局替换一拨注释
}

func (tree *AstTree) generateFile() error {
	var importPackagePart string
	if tree.importPackage != "" {
		tree.importPackage = strings.Trim(tree.importPackage, "\"")
		sli := strings.Split(tree.importPackage, "/")
		if len(sli) > 0 {
			importPackagePart = sli[len(sli) - 1]
		}
		// 外面有判断err，这里就不判断了
		modName, _ := GetCurrentModuleName()
		if strings.HasSuffix(modName, sli[0]) {
			tree.importPackage = "\"" + strings.Trim(modName, sli[0]) + tree.importPackage + "\""
		} else {
			tree.importPackage = "\"" + modName + "/" + tree.importPackage + "\""
		}
		//tree.importPackage = "\"" + modName + strings.Trim(tree.importPackage, "\"") + "\""
	}

	var dataMap = map[string]interface{}{
		"srvName":     tree.srvName,
		"pkgName":     tree.pkgName,
		"implPkgName": Visitor.PackageName,
		"routerNode":  tree.groupRouterNode,
		"importPackage": tree.importPackage,
		"defaultControllerPkgName": "controller",
		"importPackagePart": importPackagePart,
	}

	t, err := template.New("generate_all_file").Parse(CompleteRouteGenerateAndPackageTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, dataMap); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	// 文件夹不存在则创建
	dir := filepath.Dir(tree.goPath)
	dirExist := IsExist(dir)
	if !dirExist {
		// 递归创建文件夹
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	f, err := os.OpenFile(tree.goPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	return nil
}

func (tree *AstTree) generateAll() error {
	var dataMap = map[string]interface{}{
		"srvName":     tree.srvName,
		"pkgName":     tree.pkgName,
		"implPkgName": Visitor.PackageName,
		"routerNode":  tree.groupRouterNode,
	}

	t, err := template.New("generate_all_struct").Parse(CompleteRouteGenerateTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, dataMap); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	f, err := os.OpenFile(tree.goPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	return nil
}

func (tree *AstTree) generateFunc() error {
	var dataMap = map[string]interface{}{
		"srvName":     tree.srvName,
		"pkgName":     tree.pkgName,
		"implPkgName": Visitor.PackageName,
		"routerNode":  tree.groupRouterNode,
	}

	t, err := template.New("generate_func").Parse(FuncRouteGenerateTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, dataMap); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	f, err := os.OpenFile(tree.goPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	return nil
}

func (tree *AstTree) generateBindFunc() error {
	var dataMap = map[string]interface{}{
		"srvName": tree.srvName,
	}

	t, err := template.New("generate_func").Parse(FuncRouteGenerateBindFuncTpl)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	t.DefinedTemplates()
	var buf bytes.Buffer
	if err = t.Execute(&buf, dataMap); err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	f, err := os.OpenFile(tree.goPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	return nil
}

func checkRouterErrorCode(srv *proto.Service) {
	s, exist := Visitor.ImplementedRouter[srv.Name]
	if !exist {
		return
	}
	var g = genToFile{srvImplName: fmt.Sprintf("%sImpl", srv.Name), srvDetail: s}
	if err := g.parseGoFile(); err != nil {
		logrus.Errorf("parse go file err: %+v", err)
	}
}

type genToFile struct {
	srvImplName      string
	srvDetail        *positionSrv
	currentInjectRPC *injectRPC
	// rpcErrorMap      map[string]string
}

type injectRPC struct {
	protoPkg        string
	implementStruct string
	funcs           []*ast.FuncDecl
}

// parseGoFile 解析 service 对应的 gen_to 的文件
func (g *genToFile) parseGoFile() error {
	f, err := os.Open(g.srvDetail.genTo)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}

	pos := token.NewFileSet()
	astF, err := parser.ParseFile(pos, g.srvDetail.genTo, f, 0)
	if err != nil {
		logrus.Errorf("err: %+v", err)
		return err
	}
	//_ = ast.Print(pos, astF)

	for _, decl := range astF.Decls {
		switch t := decl.(type) {
		case *ast.GenDecl:
			if t.Tok.String() != "var" {
				continue
			}
			if len(t.Specs) == 0 {
				continue
			}
			def, ok := t.Specs[0].(*ast.ValueSpec)
			if !ok {
				continue
			}

			// 对应proto的名称
			var pkgName string

			defType, sok := def.Type.(*ast.SelectorExpr)
			if !sok {
				iType, iok := def.Type.(*ast.Ident)
				if !iok {
					continue
				}
				if iType.Name != g.srvImplName {
					continue
				}

				pkgName = Visitor.PackageName
			} else {
				if defType.Sel.Name != g.srvImplName {
					continue
				}

				importPkg, ok := defType.X.(*ast.Ident)
				if ok {
					pkgName = importPkg.Name
				}
			}

			// 实现了 service 的 struct
			var implName string
			if len(def.Values) == 0 {
				continue
			}
			implType, ok := def.Values[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			fun, ok := implType.Fun.(*ast.ParenExpr)
			if !ok {
				continue
			}
			xInfo, ok := fun.X.(*ast.StarExpr)
			if !ok {
				continue
			}
			xreal, ok := xInfo.X.(*ast.Ident)
			if !ok {
				continue
			}
			implName = xreal.Name
			g.currentInjectRPC = &injectRPC{
				protoPkg:        pkgName,
				implementStruct: implName,
			}
		}
	}

	if g.currentInjectRPC == nil {
		_, _ = fmt.Fprintf(os.Stderr, "get rpc nil, skip...\n")
		return nil
	}

	for _, decl := range astF.Decls {
		switch t := decl.(type) {
		case *ast.FuncDecl:
			if t.Recv == nil {
				continue
			}
			if len(t.Recv.List) == 0 {
				continue
			}
			recvList := t.Recv.List[0]
			recvType, ok := recvList.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			recvx, ok := recvType.X.(*ast.Ident)
			if !ok {
				continue
			}
			if recvx.Name != g.currentInjectRPC.implementStruct {
				continue
			}
			// 收集 implementStruct 实现的所有方法
			g.currentInjectRPC.funcs = append(g.currentInjectRPC.funcs, t)
		}
	}

	// 收集完了后 解析 funcs 的 body
	for _, decl := range g.currentInjectRPC.funcs {
		rpc, exist := g.srvDetail.rpcMap[getFuncDeclName(decl)]
		if !exist {
			continue
		}
		ok, res := iterBodyToGetCreateErrorStmt(decl.Body)
		if !ok {
			continue
		}
		ecs := repackErrorCode(g.currentInjectRPC.protoPkg, res)
		if len(ecs) == 0 {
			continue
		}
		ecs = pie.Strings(ecs).Unique().Sort()
		injectRpcErrorCodeComment(rpc, ecs)
		//logrus.Infof("rpc comment: %+v", rpc.Comment.Lines)
		//g.srvDetail.rpcMap[getFuncDeclName(decl)] = rpc
	}

	return nil
}

func injectRpcErrorCodeComment(rpc *proto.RPC, ecs []string) {
	if rpc.Comment == nil {
		rpc.Comment = new(proto.Comment)
	}
	if len(rpc.Comment.Lines) == 0 {
		var ecstring = []string{" @error:"}
		for _, s := range ecs {
			ecstring = append(ecstring, fmt.Sprintf("	%s", s))
		}
		rpc.Comment.Lines = append(rpc.Comment.Lines, ecstring...)
		return
	}
	// 检测 @error: 标签 做重置处理 只优先检测第一个 @error 标签
	var docs = rpc.Comment.Lines
	var findLabel bool
	var errLabelStart int
	for idx, line := range docs {
		reg := regexp.MustCompile(`@error:\s*(.*)`)
		if reg.MatchString(line) {
			findLabel = true
			errLabelStart = idx
			break
		}
	}
	if findLabel {
		rpc.Comment.Lines = clearErrorCodeCommentLastIndex(docs, errLabelStart)
	}

	var ecstring = []string{" @error:"}
	for _, s := range ecs {
		ecstring = append(ecstring, fmt.Sprintf("	%s", s))
	}
	rpc.Comment.Lines = append(rpc.Comment.Lines, ecstring...)
}

func clearErrorCodeCommentLastIndex(docs []string, startIndex int) []string {
	var redocs []string
	for i := 0; i < len(docs); i++ {
		if i < startIndex {
			redocs = append(redocs, docs[i])
			continue
		}
		if i == startIndex {
			continue
		}
		if !strings.Contains(docs[i], "@") && strings.Contains(docs[i], "Err") {
			continue
		}
		redocs = append(redocs, docs[i])
	}
	return redocs
}

// repackErrorCode 重新组装错误
func repackErrorCode(mainPkg string, list []ecInfo) []string {
	var ecs []string
	for _, info := range list {
		if info.pkg == mainPkg {
			ecs = append(ecs, info.name)
		} else {
			ecs = append(ecs, info.fullName)
		}
	}
	return pie.Strings(ecs).Unique()
}

// rewriteComment 重写注释
// func (g *genToFile) rewriteComment(comment string) {

// }
