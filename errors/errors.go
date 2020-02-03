package errors

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Error 自定义error类型，方便对error进行分类处理
// 比如方法调用参数非法、依赖资源访问失败，是需要能识别进行不同的处理
// API层、业务层、存储层等各层统一使用
// 约定code为2级结构
// 第一级3位
// 第二级6位，以第一级开头，对应cause字段
// 方便分类统计
type Error struct {
	Code  int    `json:"code"`   //错误码
	Msg   string `json:"msg"`    //中文错误信息,客户端展示
	MsgEn string `json:"msg_en"` //英文错误信息,客户端展示
	Log   string `json:"log"`    //不需要展示给终端，用于存放详细日志信息
	Cause *Error `json:"cause"`  //引起错误的具体错误，可以展示给终端(作为详细错误信息)
}

// Error error方法
func (e Error) Error() string {
	text, _ := json.Marshal(e)
	return string(text)
}

// WithCause 设置cause字段
func (e *Error) WithCause(cause *Error) Error {
	err := *e
	err.Cause = cause
	return err
}

// WithLog 设置log字段
func (e *Error) WithLog(log string) Error {
	err := *e
	err.Log = log
	return err
}

// IsTypeOf 校验是否是某类错误(同一种或子类型)，约定按前缀匹配
func (e *Error) IsTypeOf(code int) bool {
	return e.Code == code || strings.HasPrefix(strconv.Itoa(e.Code), strconv.Itoa(code))
}

// NewError 新建一个error对象
func NewError(code int, msg, msgEn string) Error {
	return Error{
		Code:  code,
		Msg:   msg,
		MsgEn: msgEn,
	}
}

const (
	// CodeInvalIDArgument 无效的请求参数
	CodeInvalIDArgument = 400
	// CodeUnauthorized  Unauthorized，access token没有传递、无效响应这个状态码
	CodeUnauthorized  = 401
	CodeAuthorExpired = 401
	// CodeAPINotFound API 不存在
	CodeAPINotFound = 404
	// CodeAPIMethodNotAllowed API method不允许调用
	CodeAPIMethodNotAllowed = 405
	// CodeInternalError 系统内部错误
	CodeInternalError = 500
	// CodeDataNotFound  获取不到相关内容
	CodeDataNotFound = iota + 10000
	CodeUserNotFound
	// CodeServiceBusy 服务繁忙
	CodeServiceBusy
	// CodeLockAlreadyRequired 上锁失败
	CodeLockAlreadyRequired
	// CodeNotLocalIPFound 没有找到本地ip
	CodeNotLocalIPFound
	CodePasswordErr
	CodeProjectNotExist
	CodeProjectExist
	CodeInsufficientPermissions
	CodeCron
	CodeUserExist
	CodeDontRemoveProjectAdmin
	CodeNoWorkingNode
	CodeTaskIsRunning
)

var (
	// ErrInvalidArgument 参数无效错误
	ErrInvalidArgument = Error{Code: CodeInvalIDArgument, Msg: "请求失败（错误码：400）", MsgEn: "Request failed (Error code: 400)", Log: "参数无效"}
	// ErrUnauthorized 身份认证失败请重新登录错误
	ErrUnauthorized = Error{Code: CodeUnauthorized, Msg: "身份认证失败,请重新登录", MsgEn: "Unauthorized", Log: "请求未授权"}
	// ErrAuthorExpired
	ErrAuthorExpired = Error{Code: CodeAuthorExpired, Msg: "身份认证已过期,请重新登录", MsgEn: "Unauthorized", Log: "access_token过期"}
	// ErrAPINotFound api未找到错误
	ErrAPINotFound = Error{Code: CodeAPINotFound, Msg: "请求失败（错误码：404）", MsgEn: "Request failed (Error code: 404)", Log: "方法不支持"}
	// ErrAPIMethodNotAllowed  api不允许错误
	ErrAPIMethodNotAllowed = Error{Code: CodeAPIMethodNotAllowed, Msg: "请求失败（错误码：405）", MsgEn: "Request failed (Error code: 405)", Log: "方法不允许调用"}
	// ErrInternalError api内部错误
	ErrInternalError = Error{Code: CodeInternalError, Msg: "请求失败（错误码：500）", MsgEn: "Request failed (Error code: 500)", Log: "内部错误"}
	// ErrDataNotFound 数据不存在错误
	ErrDataNotFound = Error{Code: CodeDataNotFound, Msg: "数据不存在", MsgEn: "Data not found", Log: "数据不存在"}
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = Error{Code: CodeUserNotFound, Msg: "用户不存在", MsgEn: "User not found", Log: "用户不存在"}
	// ErrCron Cron表达式错误
	ErrCron = Error{Code: CodeCron, Msg: "cron表达式错误", MsgEn: "Cron was wrong", Log: "cron表达式错误"}
	// ErrLockAlreadyRequired 抢锁失败
	ErrLockAlreadyRequired = Error{Code: CodeLockAlreadyRequired, Msg: "抢锁失败，锁已经被占用", MsgEn: "lock error, the lock already required", Log: "etcd 分布式锁抢锁失败"}
	// ErrLocalIPNotFound 没有获取到网卡ip
	ErrLocalIPNotFound = Error{Code: CodeNotLocalIPFound, Msg: "没有获取到本地IP", MsgEn: "Not local ip found", Log: "该检点没有本地网卡"}
	// ErrPasswordErr 密码错误
	ErrPasswordErr = Error{Code: CodePasswordErr, Msg: "密码错误", MsgEn: "Password is wrong", Log: "用户登录密码错误"}
	// ErrProjectNotExist 项目不存在
	ErrProjectNotExist = Error{Code: CodeProjectNotExist, Msg: "项目不存在", MsgEn: "Project not exist", Log: "项目不存在"}
	// ErrProjectExist 项目已存在
	ErrProjectExist = Error{Code: CodeProjectExist, Msg: "项目已存在", MsgEn: "Project is exist", Log: "项目已存在"}
	// ErrUserExist 项目已存在
	ErrUserExist = Error{Code: CodeUserExist, Msg: "该账号已存在", MsgEn: "User is exist", Log: "账号已存在"}
	// ErrInsufficientPermissions 权限不足
	ErrInsufficientPermissions = Error{Code: CodeInsufficientPermissions, Msg: "权限不足", MsgEn: "Insufficient permissions", Log: "无权限"}
	// ErrDontRemoveProjectAdmin 不能将管理员从项目中移除
	ErrDontRemoveProjectAdmin = Error{Code: CodeDontRemoveProjectAdmin, Msg: "无法将项目管理员移出项目", MsgEn: "Don't remove the administrator from the project", Log: "无法将项目管理员移出项目"}
	// ErrNoWorkingNode 没有工作节点
	ErrNoWorkingNode = Error{Code: CodeNoWorkingNode, Msg: "没有工作节点", MsgEn: "No working node", Log: "当前项目没有工作节点"}
	ErrTaskIsRunning = Error{Code: CodeTaskIsRunning, Msg: "任务正在执行中", MsgEn: "task is running", Log: "任务正在执行中"}
)
