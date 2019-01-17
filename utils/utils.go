package utils

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"strconv"
	"time"

	"github.com/holdno/snowFlakeByGo"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var globalIDWorker *snowFlakeByGo.Worker

// InitIDWorker 初始化ID生成器
func InitIDWorker(cluster int64) {
	var (
		err error
	)
	globalIDWorker, err = snowFlakeByGo.NewWorker(cluster)
	if err != nil {
		panic(err)
	}
}

// GetCurrentTimeText 获取当前时间format
func GetCurrentTimeText() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// BindArgsWithGin 绑定请求参数
func BindArgsWithGin(c *gin.Context, req interface{}) error {
	return c.ShouldBindWith(req, binding.Default(c.Request.Method, c.ContentType()))
}

// MakeOrderID 生成订单
func GetStrID() string {
	return strconv.FormatInt(globalIDWorker.GetId(), 10)
}

// MakeMD5 MD5加密
func MakeMD5(data string) string {
	h := md5.New()
	h.Write([]byte(data)) // 需要加密的字符串为 123456
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr) // 输出加密结果
}

// RandomStr 随机字符串
func RandomStr(l int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	seed := "1234567890QWERTYUIOPASDFGHJKLZXCVBNM"
	str := ""
	length := len(seed)
	for i := 0; i < l; i++ {
		point := r.Intn(length)
		str = str + seed[point:point+1]
	}
	return str
}

// BuildPassword 构建用户密码
func BuildPassword(password, salt string) string {
	return MakeMD5(password + salt)
}

// TernaryOperation 三元操作符
func TernaryOperation(exist bool, res, el interface{}) interface{} {
	if exist {
		return res
	}
	return el
}
