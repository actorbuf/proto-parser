package proto_parser

import (
	"fmt"

	"github.com/emicklei/proto"
	"github.com/actorbuf/iota/core"
)

type StructField struct {
	StructFieldName string
	DbFieldName     string
}

type ErrCodeInfo struct {
	ErrCode int
	ErrName string
	ErrMsg  string
}

type IndexField struct {
	Field string // 字段
	Sort  int    // 排序 1 升序 -1倒叙
}

type IndexInfo struct {
	Unique             bool          // 唯一索引
	Name               string        // 索引名称
	TTLIndex           bool          // 是否是TTL索引
	ExpireAfterSeconds int64         // 指定一个以秒为单位的数值，完成 TTL设定，设定集合的生存时间
	Fields             []*IndexField // 联合索引
}

// GroupRouterNode 组路由节点
type GroupRouterNode struct {
	FuncName   string // 函数名
	RouterPath string // 路由路径
	Method     string // 请求类型 POST/GET...
	Author     string // 接口作者
	Describe   string // 描述
	ReqName    string
	RespName   string
	rpc        *proto.RPC
	Mws        []string // 单一路由中间件
}

// GroupRouter 组路由聚合
type GroupRouter struct {
	RouterPrefix string             // 路由前缀
	GenTo        string             // 生成位置
	Apis         []*GroupRouterNode // 路由节点
	Mws          []string           // 组公共路由中间件
}

// MDocsField 字段描述
type MDocsField struct {
	FieldName  string // 字段名 对应json字段
	FieldType  string // 字段类型
	FieldDesc  string // 字段说明
	IsRequire  bool   // 是否必须
	FieldValue int    // 字段值
}

// MDocsErrCodeField rpc文档的错误码说明
type MDocsErrCodeField struct {
	Code int    // 错误码
	Pkg  string // 这个错误码所对应的包
	Name string // 错误码的名字
	Desc string // 错误码的描述
}

//go:generate pie MDocsErrCodes.SortStableUsing
type MDocsErrCodes []MDocsErrCodeField

// MDocs 文档数据
type MDocs struct {
	Node         *GroupRouterNode
	ReqName      string
	RspName      string
	Req          *proto.Message
	Rsp          *proto.Message
	ReqBody      string
	RespBody     string
	ReqFields    []*MDocsField
	RespFields   []*MDocsField
	EnumFields   map[string][]*MDocsField
	ErrCodeMap   map[string]MDocsErrCodeField
	ErrCodeList  MDocsErrCodes
	FieldJSONMap map[string]string
}

// positionSrv rpc service 位置
type positionSrv struct {
	genTo  string
	rpcMap map[string]*proto.RPC
}

type TaskNode struct {
	Desc       string // 任务描述
	Spec       string // 任务规则
	Times      int64  // 任务次数
	Type       int64  // 任务类型 0永续任务 1时间范围执行任务 2指定了执行次数的任务
	RangeStart int64  // 任务执行开始
	RangeEnd   int64  // 任务执行结束
}

type TaskConfig struct {
	GenTo string              // 任务生成位置
	Task  map[string]TaskNode //任务配置信息
}

type ProtoVisitor struct {
	dbDriver string // 数据库驱动
	CurMsg   *proto.Message
	// key 是msg嵌套层级 依靠_维持
	AllMsgMap      map[string]*proto.Message
	ModelMsgMap    map[string]*proto.Message
	OpenBsonMsgMap map[string]*proto.Message // 开启了bson支持的message
	// key是field的最终层级 依靠_维持 value是bson_tag或者原字段名称(没找到直接按照原名称写入)
	BsonTagMap map[string]string
	// 包名
	PackageName string
	// MDocDepPkgName 文档依赖的包名
	MDocDepPkgName string
	// 注入的table_name map
	ModelTableNameMap map[string]string
	// 注入err_code
	ErrCodeList []*ErrCodeInfo
	// model字段映射
	ModelFieldStructMap map[string]map[string]ModelFieldStruct
	// 索引
	ModelIndexMap map[string]*IndexInfo
	// 路由注册 srvName:GroupRouter
	GroupRouterMap map[string]*GroupRouter
	// 路由注册 导入哪些包
	GroupRouterImportPkg []string
	// MD输出
	MDoc *MDocs
	// MD输出时依赖message列表 package=>message_name=>message
	MDocDepMessageMap map[string]map[string]*proto.Message
	MDocDepEnum       map[string]map[string]*proto.Enum
	// api对应的srv列表
	APIGroupSrvMap map[string]*proto.Service
	SrvMap         map[string]*proto.Service
	AllEnumMap     map[string]*proto.Enum
	// 已实现的路由 srv:rpc_name:rpc
	ImplementedRouter map[string]*positionSrv
	// 当前处理到的枚举类型
	ErrCodeEnum *proto.Enum
	// 所有错误字段 pkg.errCode: errorCode
	ErrCodeEnumFieldMap map[string]MDocsErrCodeField
	// 函数名=>注释
	FuncCommentMap map[string]string
	// 限频路径=>struct{}
	FreqMap core.FreqMap
	// 任务配置
	Tasks map[string]TaskConfig
}

// AddTask 添加任务
func (p *ProtoVisitor) AddTask(task, taskName, genTo string, config TaskNode) {
	if len(p.Tasks) == 0 {
		p.Tasks = make(map[string]TaskConfig)
	}
	_, ok := p.Tasks[task]
	if !ok {
		p.Tasks[task] = TaskConfig{
			GenTo: genTo,
			Task:  map[string]TaskNode{},
		}
	}
	p.Tasks[task].Task[taskName] = config
}

// AddFreq 添加限频规则
func (p *ProtoVisitor) AddFreq(key string, c core.FreqConfig) {
	if len(p.FreqMap) == 0 {
		p.FreqMap = make(map[string]core.FreqConfig)
	}
	p.FreqMap[key] = c
}

// AddFuncComment 给函数添加注释覆盖
func (p *ProtoVisitor) AddFuncComment(name string, comment string) {
	if len(p.FuncCommentMap) == 0 {
		p.FuncCommentMap = make(map[string]string)
	}
	p.FuncCommentMap[name] = comment
}

// AddMDocDepMessage 添加依赖message的依赖列表
func (p *ProtoVisitor) AddMDocDepMessage(name string, m *proto.Message) {
	if len(p.MDocDepMessageMap) == 0 {
		p.MDocDepMessageMap = make(map[string]map[string]*proto.Message)
	}
	if _, exist := p.MDocDepMessageMap[name]; !exist {
		p.MDocDepMessageMap[name] = make(map[string]*proto.Message)
	}
	p.MDocDepMessageMap[name][m.Name] = m
}

// AddMDocDepEnum 添加依赖message的依赖列表
func (p *ProtoVisitor) AddMDocDepEnum(name string, m *proto.Enum) {
	if len(p.MDocDepEnum) == 0 {
		p.MDocDepEnum = make(map[string]map[string]*proto.Enum)
	}
	if _, exist := p.MDocDepEnum[name]; !exist {
		p.MDocDepEnum[name] = make(map[string]*proto.Enum)
	}
	p.MDocDepEnum[name][m.Name] = m
}

// AddErrCodeEnumField 添加错误码
func (p *ProtoVisitor) AddErrCodeEnumField(name string, field MDocsErrCodeField) {
	if len(p.ErrCodeEnumFieldMap) == 0 {
		p.ErrCodeEnumFieldMap = make(map[string]MDocsErrCodeField)
	}
	p.ErrCodeEnumFieldMap[name] = field
}

func (p *ProtoVisitor) AddImplRouter(srv, to string, rpc *proto.RPC) {
	if len(p.ImplementedRouter) == 0 {
		p.ImplementedRouter = make(map[string]*positionSrv)
	}
	if p.ImplementedRouter[srv] == nil {
		p.ImplementedRouter[srv] = new(positionSrv)
	}
	p.ImplementedRouter[srv].genTo = to
	if p.ImplementedRouter[srv].rpcMap == nil {
		p.ImplementedRouter[srv].rpcMap = make(map[string]*proto.RPC)
	}
	p.ImplementedRouter[srv].rpcMap[rpc.Name] = rpc
}

func (p *ProtoVisitor) AddDocJSONMap(fieldName, jsonName string) {
	if p.MDoc == nil {
		p.MDoc = new(MDocs)
	}
	if len(p.MDoc.FieldJSONMap) == 0 {
		p.MDoc.FieldJSONMap = make(map[string]string)
	}
	p.MDoc.FieldJSONMap[fieldName] = jsonName
}

func (p *ProtoVisitor) AddDocEnum(enumName string, docs []*MDocsField) {
	if p.MDoc == nil {
		p.MDoc = new(MDocs)
	}
	if len(p.MDoc.EnumFields) == 0 {
		p.MDoc.EnumFields = make(map[string][]*MDocsField)
	}
	p.MDoc.EnumFields[enumName] = docs
}

func (p *ProtoVisitor) AddEnum(enumName string, e *proto.Enum) {
	if len(p.AllEnumMap) == 0 {
		p.AllEnumMap = make(map[string]*proto.Enum)
	}
	p.AllEnumMap[enumName] = e
}

func (p *ProtoVisitor) AddSrv(srvName string, srv *proto.Service) {
	if len(p.SrvMap) == 0 {
		p.SrvMap = make(map[string]*proto.Service)
	}
	p.SrvMap[srvName] = srv
}

func (p *ProtoVisitor) AddApiSrv(srvName string, srv *proto.Service) {
	if len(p.APIGroupSrvMap) == 0 {
		p.APIGroupSrvMap = make(map[string]*proto.Service)
	}
	p.APIGroupSrvMap[srvName] = srv
}

func (p *ProtoVisitor) AddRouterGroup(srvName string, gr *GroupRouter) {
	if len(p.GroupRouterMap) == 0 {
		p.GroupRouterMap = make(map[string]*GroupRouter)
	}
	p.GroupRouterMap[srvName] = gr
}

func (p *ProtoVisitor) AddIndexField(name string, field *IndexField) error {
	if len(p.ModelIndexMap) == 0 {
		p.ModelIndexMap = make(map[string]*IndexInfo)
	}

	var info = p.ModelIndexMap[name]
	if info == nil {
		info = &IndexInfo{
			Name: name,
		}
		info.Fields = append(info.Fields, field)
		p.ModelIndexMap[name] = info
		return nil
	}

	// 唯一索引定义前后不一致 抛错
	if info.Unique {
		return fmt.Errorf("unique_index unified: %s", name)
	}

	// ttl索引设置多个值 抛错
	if info.TTLIndex {
		return fmt.Errorf("multi ttl_index: %s", name)
	}

	info.Fields = append(info.Fields, field)

	p.ModelIndexMap[name] = info
	return nil
}

func (p *ProtoVisitor) AddUniqueIndexField(name string, field *IndexField) error {
	if len(p.ModelIndexMap) == 0 {
		p.ModelIndexMap = make(map[string]*IndexInfo)
	}

	var info = p.ModelIndexMap[name]
	if info == nil {
		info = &IndexInfo{
			Unique: true,
			Name:   name,
		}
		info.Fields = append(info.Fields, field)
		p.ModelIndexMap[name] = info
		return nil
	}

	// 唯一索引定义前后不一致 抛错
	if !info.Unique {
		return fmt.Errorf("unique_index unified: %s", name)
	}

	// ttl索引设置多个值 抛错
	if info.TTLIndex {
		return fmt.Errorf("multi ttl_index: %s", name)
	}

	info.Fields = append(info.Fields, field)

	p.ModelIndexMap[name] = info
	return nil
}

func (p *ProtoVisitor) AddTTLIndexField(name string, expiredTime int64, field *IndexField) error {
	if len(p.ModelIndexMap) == 0 {
		p.ModelIndexMap = make(map[string]*IndexInfo)
	}

	var info = p.ModelIndexMap[name]
	if info == nil {
		info = &IndexInfo{
			Name:               name,
			TTLIndex:           true,
			ExpireAfterSeconds: expiredTime,
		}
		info.Fields = append(info.Fields, field)
		p.ModelIndexMap[name] = info
		return nil
	}

	// 唯一索引定义前后不一致 抛错
	if info.Unique {
		return fmt.Errorf("unique_index unified: %s", name)
	}

	// ttl索引设置多个值 抛错
	if info.TTLIndex {
		return fmt.Errorf("multi ttl_index: %s", name)
	}

	info.Fields = append(info.Fields, field)

	p.ModelIndexMap[name] = info
	return nil
}

func (p *ProtoVisitor) AddErrCode(code int, name, msg string) {
	var eci = &ErrCodeInfo{
		ErrCode: code,
		ErrName: name,
		ErrMsg:  msg,
	}
	p.ErrCodeList = append(p.ErrCodeList, eci)
}

func (p *ProtoVisitor) AddModelTableName(key, value string) {
	if len(p.ModelTableNameMap) == 0 {
		p.ModelTableNameMap = make(map[string]string)
	}
	p.ModelTableNameMap[key] = value
}

func (p *ProtoVisitor) AddBsonTag(key, value string) {
	if len(p.BsonTagMap) == 0 {
		p.BsonTagMap = make(map[string]string)
	}

	p.BsonTagMap[key] = value
}

func (p *ProtoVisitor) AddMsg(fullPath string, m *proto.Message) {
	if len(p.AllMsgMap) == 0 {
		p.AllMsgMap = make(map[string]*proto.Message)
	}

	p.AllMsgMap[fullPath] = m
}

func (p *ProtoVisitor) AddModelMsg(fullPath string, m *proto.Message) {
	if len(p.ModelMsgMap) == 0 {
		p.ModelMsgMap = make(map[string]*proto.Message)
	}

	p.ModelMsgMap[fullPath] = m
}

func (p *ProtoVisitor) VisitMessage(m *proto.Message) {
	panic("implement VisitMessage")
}

func (p *ProtoVisitor) VisitService(v *proto.Service) {
	panic("implement VisitService")
}

func (p *ProtoVisitor) VisitSyntax(s *proto.Syntax) {
	panic("implement VisitSyntax")
}

func (p *ProtoVisitor) VisitPackage(pkg *proto.Package) {
	panic("implement VisitPackage")
}

func (p *ProtoVisitor) VisitOption(o *proto.Option) {
	panic("implement VisitOption")
}

func (p *ProtoVisitor) VisitImport(i *proto.Import) {
	panic("implement VisitImport")
}

func (p *ProtoVisitor) VisitNormalField(i *proto.NormalField) {
	if p.CurMsg == nil {
		return
	}
}

func (p *ProtoVisitor) VisitEnumField(i *proto.EnumField) {
	panic("implement VisitEnumField")
}

func (p *ProtoVisitor) VisitEnum(e *proto.Enum) {
	panic("implement VisitEnum")
}

func (p *ProtoVisitor) VisitComment(e *proto.Comment) {
	panic("implement VisitComment")
}

func (p *ProtoVisitor) VisitOneof(o *proto.Oneof) {
	panic("implement VisitOneof")
}

func (p *ProtoVisitor) VisitOneofField(o *proto.OneOfField) {
	panic("implement VisitOneofField")
}

func (p *ProtoVisitor) VisitReserved(r *proto.Reserved) {
	panic("implement VisitReserved")
}

func (p *ProtoVisitor) VisitRPC(r *proto.RPC) {
	panic("implement VisitRPC")
}

func (p *ProtoVisitor) VisitMapField(f *proto.MapField) {
	panic("implement VisitMapField")
}

func (p *ProtoVisitor) VisitGroup(g *proto.Group) {
	panic("implement VisitGroup")
}

func (p *ProtoVisitor) VisitExtensions(e *proto.Extensions) {
	panic("implement VisitExtensions")
}
