package utils

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/holdno/snowFlakeByGo"
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

// Random 生成随机数
func Random(min, max int) int {
	if min == max {
		return max
	}
	max = max + 1
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return min + r.Intn(max-min)
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

// GetContextWithTimeout 返回一个带timeout的context
func GetContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.TODO(), time.Duration(config.GetServiceConfig().Deploy.Timeout)*time.Second)
}

// GetBeforeDate 获取n天前的时间
func GetDateFromNow(n int) time.Time {
	timer, _ := time.ParseInLocation("2006-01-02", time.Now().Format("2006-01-02"), time.Local)
	if n == 0 {
		return timer
	}
	return timer.AddDate(0, 0, n)
}

// 获取机器ip
func GetLocalIP() (string, error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		err     error
		ipNet   *net.IPNet
		isIpNet bool
	)

	if addrs, err = net.InterfaceAddrs(); err != nil {
		return "", err
	}

	// 获取第一个非IO的网卡
	for _, addr = range addrs {
		// ipv4  ipv6
		// 如果能反解成ip地址 则为我们需要的地址
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			// 是ip地址 不是 unix socket地址
			// 继续判断 是ipv4 还是 ipv6
			// 跳过ipv6
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", errors.ErrLocalIPNotFound
}

// StrArrExist 检测string数组中是否包含某个字符串
func StrArrExist(arr []string, check string) bool {
	for _, v := range arr {
		if v == check {
			return true
		}
	}
	return false
}

func GetUserID(c *gin.Context) int64 {
	return c.GetInt64(common.USER_ID)
}

// RetryFunc 带重试的func
func RetryFunc(times int, f func() error) error {
	var (
		reTimes int
		err     error
	)
RETRY:
	if err = f(); err != nil {
		if reTimes == times {
			return err
		}
		time.Sleep(time.Duration(1) * time.Second)
		reTimes++
		goto RETRY
	}
	return nil
}
