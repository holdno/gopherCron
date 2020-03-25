package response

import (
	"net/http"
	"time"

	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/errors"
	log "github.com/sirupsen/logrus"
)

// 常量定义
const (
	RequestIDKey = "request_id"
	ResponseKey  = "response_key"
)

// Response 响应结构体定义
type Response struct {
	Meta *Meta       `json:"meta"`
	Body interface{} `json:"response"`
}

// Meta 响应meta定义
type Meta struct {
	errors.Error
	RequestID  string `json:"request_id"`
	RequestURI string `json:"request_uri"`
}

// BodyPaging 分页参数结构
type BodyPaging struct {
	Cursors BodyCursors `json:"cursors"`
}

// BodyCursors 分页参数
type BodyCursors struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// APIError api响应失败
func APIError(c *gin.Context, err error) {
	res := c.MustGet(ResponseKey).(*Response)
	cerr, ok := err.(errors.Error)
	if !ok {
		if cerrptr, ok := err.(*errors.Error); !ok {
			res.Meta.Error = errors.ErrInternalError
			res.Meta.Error.Log = err.Error()
		} else {
			res.Meta.Error = *cerrptr
		}
	} else {
		res.Meta.Error = cerr
	}

	if res.Meta.Error.Code < 600 {
		c.JSON(res.Meta.Error.Code, res)
	} else {
		c.JSON(http.StatusOK, res)
	}

	printLog(c, res.Meta)
}

func printLog(c *gin.Context, meta *Meta) {
	// 统一打印日志
	logFields := log.Fields{
		"RequestID":  meta.RequestID,
		"RequestURI": c.Request.URL.Path,
		"EndTime":    time.Now().UnixNano(),
		"Code":       meta.Code,
		"ErrMsg":     meta.Msg,
		"Log":        meta.Log,
	}

	if c.Request.Method == "POST" {
		logFields["Params"] = c.Request.PostForm.Encode()
	} else {
		logFields["Params"] = c.Request.URL.Query().Encode()
	}

	log.WithFields(logFields).Info("api request error~")
}

func printSuccessLog(c *gin.Context, meta *Meta) {
	// 统一打印日志
	logFields := log.Fields{
		"RequestID":  meta.RequestID,
		"RequestURI": c.Request.URL.Path,
		"EndTime":    time.Now().UnixNano(),
		"Code":       meta.Code,
		"Log":        meta.Log,
	}

	if c.Request.Method == "POST" {
		logFields["Params"] = c.Request.PostForm.Encode()
	} else {
		logFields["Params"] = c.Request.URL.Query().Encode()
	}

	log.WithFields(logFields).Info("successful api request ~")
}

// APISuccess api响应成功
func APISuccess(c *gin.Context, response interface{}) {
	res := c.MustGet(ResponseKey).(*Response)

	if response != nil {
		res.Body = response
	}

	c.JSON(http.StatusOK, res)
	printSuccessLog(c, res.Meta)
}

// GetRequestID 获取请求ID
func GetRequestID(c *gin.Context) string {
	reqIDValue, isExist := c.Get(RequestIDKey)
	var reqID string
	if isExist {
		reqID = reqIDValue.(string)
	} else {
		//生成RequestID
		reqID = utils.GetStrID()
		c.Set(RequestIDKey, reqID)
	}
	return reqID
}
