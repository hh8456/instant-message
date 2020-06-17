package main

import (
	"encoding/json"
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/models"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

type versionInfo struct {
	HttpCode  int    // 回复码
	LogicCode int    // 0 强制更新, 1 选择更新, 2 不用更新
	UpdateUrl string // 更新地址
	Describe  string // 描述
}

var (
	version           models.Version
	iosMinVersion     int64 // ios 最小版本号
	iosMaxVersion     int64 // ios 最大版本号
	androidMinVersion int64 // android 最小版本号
	androidMaxVersion int64 // android 最大版本号
)

func loadDb() {
	imPublicConfig, err1 := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err1 {
		str := fmt.Sprintf("读取配置 public.json 文件出错:%v", err1)
		panic(str)
	}

	println(imPublicConfig.Mysql)
	orm := function.Must(xorm.NewEngine("mysql", imPublicConfig.Mysql)).(*xorm.Engine)
	orm.SetMaxIdleConns(5)
	orm.SetMaxOpenConns(10)
	orm.SetConnMaxLifetime(time.Minute * 14)
	function.Must(nil, orm.Ping())

	orm.SQL("select * from version order by `id` desc limit 1").Get(&version)

	//println(version.IosUpdateUrl)
	println(version.AndroidUpdateUrl)
	var e error
	iosMinVersion, e = strconv.ParseInt(strings.Replace(version.IosMinVersion, ".", "", -1), 10, 64)
	function.Must(nil, e)
	iosMaxVersion, e = strconv.ParseInt(strings.Replace(version.IosMaxVersion, ".", "", -1), 10, 64)
	function.Must(nil, e)
	androidMinVersion, e = strconv.ParseInt(strings.Replace(version.AndroidMinVersion, ".", "", -1), 10, 64)
	function.Must(nil, e)
	androidMaxVersion, e = strconv.ParseInt(strings.Replace(version.AndroidMaxVersion, ".", "", -1), 10, 64)
	function.Must(nil, e)

	fmt.Println("vim-go")
}

// 跨域
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method               //请求方法
		origin := c.Request.Header.Get("Origin") //请求头部
		var headerKeys []string                  // 声明请求头keys
		for k, _ := range c.Request.Header {
			headerKeys = append(headerKeys, k)
		}
		headerStr := strings.Join(headerKeys, ", ")
		if headerStr != "" {
			headerStr = fmt.Sprintf("access-control-allow-origin, access-control-allow-headers, %s", headerStr)
		} else {
			headerStr = "access-control-allow-origin, access-control-allow-headers"
		}
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Origin", "*")                                       // 这是允许访问所有域
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE") //服务器支持的所有跨域请求的方法,为了避免浏览次请求的多次'预检'请求
			//  header的类型
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,X_Requested_With,Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language,DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma")
			//				允许跨域设置																										可以返回其他子段
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar") // 跨域关键设置 让浏览器可以解析
			c.Header("Access-Control-Max-Age", "172800")                                                                                                                                                           // 缓存请求信息 单位为秒
			c.Header("Access-Control-Allow-Credentials", "false")                                                                                                                                                  //	跨域请求是否需要带cookie信息 默认设置为true
			c.Set("content-type", "application/json")                                                                                                                                                              // 设置返回格式是json
		}

		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.JSON(http.StatusOK, "Options Request!")
		}
		// 处理请求
		c.Next() //	处理请求
	}
}

func main() {
	loadDb()
	r := gin.Default()
	r.Use(Cors())
	Cors()

	// GET /login?cellphonenumber=1234&verificode=5678
	r.GET("/ios", ios)
	r.GET("/android", android)
	r.Run("0.0.0.0:6789")
}

func parseClientVersion(strVer string) (int64, error) {
	clientVersion, e := strconv.ParseInt(strings.Replace(strVer, ".", "", -1), 10, 64)
	if e != nil {
		return 0, e
	}

	return clientVersion, nil
}

// GET /ios?version=1.2.3
func ios(c *gin.Context) {
	ack := &versionInfo{}
	cliVer := c.Query("version")
	clientVersion, e := parseClientVersion(cliVer)
	if e != nil {
		ack.HttpCode = http.StatusBadRequest
		r, _ := json.Marshal(ack)
		c.Writer.Write(r)
		return
	}

	// 强制更新
	if clientVersion < iosMinVersion {
		ack.LogicCode = 0
	}

	//选择更新
	if clientVersion >= iosMinVersion && clientVersion <= iosMaxVersion {
		ack.LogicCode = 1
	}

	// 不更新
	if clientVersion > iosMaxVersion {
		ack.LogicCode = 2
	}

	ack.UpdateUrl = version.IosUpdateUrl
	ack.Describe = version.IosUpdateDescribe
	ack.HttpCode = http.StatusOK

	r, _ := json.Marshal(ack)
	c.Writer.Write(r)

}

// GET /android?version=1.2.3
func android(c *gin.Context) {
	ack := &versionInfo{}
	cliVer := c.Query("version")
	clientVersion, e := parseClientVersion(cliVer)
	if e != nil {
		// XXX log
		ack.HttpCode = http.StatusBadRequest
		r, _ := json.Marshal(ack)
		c.Writer.Write(r)
		return
	}

	// 强制更新
	if clientVersion < androidMinVersion {
		ack.LogicCode = 0
	}

	//选择更新
	if clientVersion >= androidMinVersion && clientVersion < androidMaxVersion {
		ack.LogicCode = 1
	}

	// 不更新
	if clientVersion >= androidMaxVersion {
		ack.LogicCode = 2
	}

	ack.UpdateUrl = version.AndroidUpdateUrl
	ack.Describe = version.AndroidUpdateDescribe
	ack.HttpCode = http.StatusOK

	r, _ := json.Marshal(ack)
	c.Writer.Write(r)
}
