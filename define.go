package proto_parser

// 全局变量相关
var (
	// ModelTplNotGenerateGetScopeFunc 控制 ModelTpl 模板不生成 GetScope 函数
	ModelTplNotGenerateGetScopeFunc bool
	// FreqRuleOutput 限频规则输出路径
	FreqRuleOutput string
	// PbFilePath proto 文件位置
	PbFilePath string
)

// Message 名字前后缀相关
const (
	ErrCodeName  = "ErrCode"
	NameModel    = "Model"
	NameAPIGroup = "API"
	NameReq      = "Req"
	NameResp     = "Resp"
	NameGormTag  = "@gorm"
)

// 正则相关
const (
	RegexpBson              = "@bson:(\\s)*([a-zA-Z0-9_-]+)"
	RegexpJson              = "@json:[\\s]*([a-zA-Z0-9_-]+)"
	RegexpGorm              = "@gorm:[\\s]*([a-zA-Z0-9_\\-:<>;\\(\\)\\s]+)"
	RegexpJsonStyle         = "@json_style:\\s*([a-zA-Z0-9_]+)"
	RegexpGroupRouter       = "@route_group:\\s*([\\w]*)"
	RegexpGroupRouterAPI    = "@route_api:\\s*([\\w|/]*)"
	RegexpRouterGenTo       = "@gen_to:\\s*([\\w|/|\\.]*)"
	RegexpRouterRpcAuthor   = "@author:\\s*(.*)"
	RegexpRouterRpcDesc     = "@desc:\\s*(.*)"
	RegexpRouterRpcMethod   = "@method:\\s*([\\w]*)"
	RegexpRouterRpcURL      = "@api:\\s*([\\w|/]*)"
	RegexpAddModel          = "@model:\\s*true"
	RegexpMiddlewareContent = "@middleware:[\\s]*([^\\s].*)"
	RegexpMiddlewareFunc    = "([a-zA-Z0-9_/\\-]*)\\[(.*?)*\\]"
	RegexpRpcGen            = "@rpc_gen:\\s*true"
	RegexpFreq              = "@freq:[\\s]*(\\d+\\s\\d+\\s\\d+)"
	RegexpTask              = "@task:\\s*true"
	RegexpTaskTimeSpec      = "@t:\\s*(.*)"
	RegexpTaskTimes         = "@times:\\s*(\\d*)"
	RegexpTaskRange         = "@range:\\s*([\\d]* [\\d]*)"
	RegexpTaskType          = "@type:\\s*(\\d)"
)

type ModelFieldStruct struct {
	StructFieldName string
	DbFieldName     string
	Comment         string
}

type CodeGenConfig struct {
	PbFilePath       string   // 需要生成的proto文件路径 支持目录但不支持正则
	OutputPath       string   // 代码需要生成到什么位置
	GrpcOutputPath   string   // grpc代码需要生成到什么位置
	IncludePbFiles   []string // 生成的代码引用到的其他proto文件列表
	OutputNeedFormat bool     // 生成位置的代码是否需要用 gofmt 格式化一下
	NoGetScopeFunc   bool     // 生成的代码不要包含 mdbc 的 GetScope 函数
	DbDriveType      string   // 数据库驱动类型
	FreqOutput       string   // 限频文件输出路径
}
