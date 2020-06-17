package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/common-library/snowflake"
	"instant-message/msg-server/client"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/hh8456/go-common/redisObj"
	"github.com/hh8456/redisSession"
	uuid "github.com/satori/go.uuid"
)

const (
	tokenKey                   = "imTokenKey"
	DefaultAgentPassword       = "1234567888"
	DefaultThirdPlatformId     = 1000000
	DefaultThirdPlatformUserId = 100000000
)

// 管理帐号权限
const (
	CHANGE_CONFIG                 = 1  // 0000 0001	修改平台配置
	QUERY_MANAGER_ACCOUNT         = 2  // 0000 0010	查询管理员帐号
	CHANGE_OTHER_MANAGER_PASSWORD = 4  // 0000 0100	修改其他管理员帐号密码
	CHANGE_OTHER_MANAGER_STATUS   = 8  // 0000 1000	修改其他管理员状态（禁用、停用）
	REGISTER_MANAGER_ACCOUNT      = 16 // 0001 0000	注册管理员帐号
)

func init() {
	// 建立日志文件
	logDir := os.Getenv("IMLOGPATH")
	exits, err := function.PathExists(logDir)
	if err != nil {
		str := fmt.Sprintf("get dir error, %v", err)
		log.Errorf(str)
		panic(str)
	}

	if !exits {
		os.Mkdir(logDir, os.ModePerm)
	}

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/manager", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

type ImManager struct {
	orm                *xorm.Engine
	rdsUser            *redisSession.RedisSession
	rdsManager         *redisSession.RedisSession
	rdsThirdPlatform   *redisSession.RedisSession
	rdsRegionRegular   *redisSession.RedisSession
	rdsLoginToken      *redisSession.RedisSession
	rdsRegisterAccount *redisSession.RedisSession
	rdsAgent           *redisSession.RedisSession
	rdsBriefInfo       *redisSession.RedisSession
	httpSrv            *http.Server
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

func NewImManager(mysqlConfig string) *ImManager {
	imManagerEngine := gin.Default()

	// 允许使用跨域请求
	imManagerEngine.Use(Cors())
	Cors()

	imMgr := &ImManager{
		orm:                function.Must(xorm.NewEngine("mysql", mysqlConfig)).(*xorm.Engine),
		rdsUser:            redisObj.NewSessionWithPrefix(client.RdsKeyPrefixUserOnline),
		rdsManager:         redisObj.NewSessionWithPrefix(client.RdsKeyPrefixManagerLoginString),
		rdsThirdPlatform:   redisObj.NewSessionWithPrefix(client.RdsKeyPrefixThirdPlatformSet),
		rdsRegionRegular:   redisObj.NewSessionWithPrefix(client.RdsKeyPrefixRegionRegularString),
		rdsLoginToken:      redisObj.NewSessionWithPrefix(client.RdsKeyPrefixThirdPlatformLoginToken),
		rdsRegisterAccount: redisObj.NewSessionWithPrefix(client.RdsKeyRegisterAccount),
		rdsAgent:           redisObj.NewSessionWithPrefix(client.RdsKeyPrefixHighAgentSet),
		rdsBriefInfo:       redisObj.NewSessionWithPrefix(client.RdsKeyPrefixBriefInfo),
		httpSrv: &http.Server{
			Addr:    "0.0.0.0:8085",
			Handler: imManagerEngine,
		},
	}

	/* 管理员帐号 */
	/* 管理帐号登录 */
	imManagerEngine.GET("/login", imMgr.Login)
	/* 管理账号修改密码 */
	imManagerEngine.POST("/changePassword", imMgr.ChangePassword)
	/* 管理员帐号启用\停用 */
	imManagerEngine.POST("/changeStatus", imMgr.ChangeStatus)
	/* 管理员帐号列表 */
	imManagerEngine.GET("/getManagerList", imMgr.GetManagerList)
	/* 新建管理员帐号 */
	imManagerEngine.POST("/newManager", imMgr.NewManager)

	/* 权限控制 */
	/* 设置手机号获取验证码的地区-正则匹配配置 */
	imManagerEngine.POST("/setRegionRegularConfig", imMgr.SetRegionRegularConfig)
	/* 查看手机号获取验证码的地区-正则匹配配置 */
	imManagerEngine.GET("/queryRegionRegularConfig", imMgr.QueryRegionRegularConfig)

	/* IM 统计 */
	/* 获取当前在线用户 */
	imManagerEngine.GET("/getOnlineUser", imMgr.GetOnlineUser)
	/* 按天获取活跃用户 */
	imManagerEngine.GET("/getLoginUser", imMgr.GetLoginUser)
	/* 按天获取注册用户 */
	imManagerEngine.GET("/getRegisterUser", imMgr.GetRegisterUser)
	/* 按天获取活跃用户数 */
	imManagerEngine.GET("/getLoginUserNum", imMgr.GetLoginUserNum)
	/* 按天获取注册用户数 */
	imManagerEngine.GET("/getRegisterUserNum", imMgr.GetRegisterUserNum)
	/* 获取更新设置 */
	imManagerEngine.GET("/getUpdateSet", imMgr.GetUpdateSet)
	/* 更新设置 */
	imManagerEngine.POST("/updateSet", imMgr.UpdateSet)
	/* 修改帐号状态 */
	imManagerEngine.POST("/changeAccountStatus", imMgr.ChangeAccountStatus)
	/* 重置帐号密码 */
	imManagerEngine.POST("/resetAccountPassword", imMgr.ResetAccountPassword)

	/* 第三方平台 XXX 已联调通过, 待测试*/
	/* 注册第三方平台 */
	imManagerEngine.POST("/registerThirdPlatform", imMgr.RegisterThirdPlatform)
	/* 查看已注册的第三方平台 */
	imManagerEngine.GET("/queryThirdPlatform", imMgr.QueryThirdPlatform)
	/* 禁用第三方平台 */
	imManagerEngine.POST("/forbidThirdPlatform", imMgr.ForbidThirdPlatform)
	/* 第三方平台玩家帐号登录，请求token */
	imManagerEngine.POST("/getPlayerToken", imMgr.GetPlayerToken)

	return imMgr
}

type jwtCustomClaims struct {
	JwtStandardClaims *jwt.StandardClaims `json:"jwt_standard_claims"`

	// append extra message
	SnowFlakeId int64 `json:"snow_flake_id"`
}

func (claims jwtCustomClaims) Valid() error {
	fmt.Println(runtime.Caller(0))
	return nil
}

// CreateToken 通过jwt算法生成token
func CreateToken(SecretKey []byte, username string, accountId, timeout, snowFlakeId int64) (string, error) {
	claims := &jwtCustomClaims{
		JwtStandardClaims: &jwt.StandardClaims{
			Audience:  username,                         // 标识token的接收者
			ExpiresAt: timeout,                          // token过期时间
			Id:        strconv.FormatInt(accountId, 10), // 自定义的id号
			IssuedAt:  time.Now().Unix(),                // 签名的发行时间
			Issuer:    "",                               // 签名的发行者
			NotBefore: 0,                                // 这条token信息生效时间.这个值可以不设置,但是设定后,一定要大于当前Unix UTC,否则token将会延迟生效
			Subject:   "",                               // 签名面向的用户
		},
		SnowFlakeId: snowFlakeId,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(SecretKey)
}

// CheckToken 通过jwt算法校验token
func CheckToken(c *gin.Context) (*jwt.Token, error) {
	tokenSrt := c.Request.Header.Get("token")
	if "" == tokenSrt {
		return nil, errors.New("token 为空")
	}

	return jwt.Parse(tokenSrt,
		func(*jwt.Token) (i interface{}, e error) {
			return []byte(tokenKey), nil
		},
	)
}

// Login 管理帐号登录
func (imMgr *ImManager) Login(c *gin.Context) {
	userName := c.Query("username")
	passWord := c.Query("password")
	manager := &models.Manager{}
	bRet, err := imMgr.orm.Where("username = ?", userName).Get(manager)
	if nil != err || !bRet {
		log.Warnf("用户%s登录时，用户不存在", userName)
		c.String(http.StatusBadRequest, "用户名或密码错误")
		return
	}

	if passWord != manager.Password {
		log.Warnf("用户%s登录时，密码错误", userName)
		c.String(http.StatusBadRequest, "用户名或密码错误")
		return
	}

	if 1 == manager.Status {
		c.String(http.StatusBadRequest, "此账号已停用")
		return
	}

	token, err := CreateToken([]byte(tokenKey), manager.Username, int64(manager.Id), time.Now().Add(time.Hour*72).Unix(), snowflake.GetSnowflakeId())
	if nil != err {
		log.Errorf("用户%s登录时，生成Token失败:%v", userName, err)
		c.String(http.StatusInternalServerError, "服务器生成Token失败，请重试")
		return
	}

	tm := time.Unix(time.Now().Unix(), 0)
	timeStr := tm.Format("2006-01-02 15:04:05")
	_, err = imMgr.orm.DB().Exec("UPDATE manager SET last_login_time = ? WHERE username = ? AND password = ?", timeStr, userName, passWord)
	if nil != err {
		log.Warnf("用户%s登录时，更新最近登录时间错误%v", userName, err)
	}

	err = imMgr.rdsManager.Setex(token, time.Hour*1, userName)
	if nil != err {
		log.Warnf("用户%s登录，token存入redis失败%v，修改密码可能出错", userName, err)
	}

	c.String(http.StatusOK, token)
}

type ChangePasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ChangePassword 管理账号修改密码
func (imMgr *ImManager) ChangePassword(c *gin.Context) {
	/* 校验token和参数 */
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v修改管理员密码,获取用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	requestBody := &ChangePasswordRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	if nil != err {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	username := requestBody.Username
	password := requestBody.Password
	if "" == username || "" == password {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	/* 校验权限 */
	if username != userName {
		accountAuthority := imMgr.GetManagerAccountAuthority(userName)
		if 0 == accountAuthority {
			log.Error("用户%s修改管理员密码，获取权限失败", userName)
			c.String(http.StatusInternalServerError, "获取权限失败")
			return
		}

		if 0 == accountAuthority&CHANGE_OTHER_MANAGER_PASSWORD {
			c.String(http.StatusBadRequest, "权限不足")
			return
		}
	}

	/* 修改密码 */
	_, err = imMgr.orm.DB().Exec("UPDATE manager SET password = ? WHERE username = ?", password, userName)
	if nil != err {
		log.Error("用户%s修改管理员密码，修改数据库失败%v", userName, err)
		c.String(http.StatusInternalServerError, "修改数据库失败")
		return
	}

	c.String(http.StatusOK, "修改成功")
}

type ChangeStatusRequest struct {
	Username string `json:"username"`
	Status   int    `json:"status"`
}

// ChangeStatus 管理员账号修改状态
func (imMgr *ImManager) ChangeStatus(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v修改管理员状态,获取用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	requestBody := &ChangeStatusRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	if nil != err {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	username := requestBody.Username
	status := requestBody.Status
	if "" == username || (1 != status && 0 != status) {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	_, err = imMgr.orm.DB().Exec("UPDATE manager SET status = ? WHERE username = ?", status, username)
	if nil != err {
		log.Error("用户%s修改管理员状态,修改数据库出错%v", userName, err)
		c.String(http.StatusInternalServerError, "修改数据库出错")
		return
	}

	c.String(http.StatusOK, "修改成功")
}

type managerListResponse struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	LastLoginTime string `json:"last_login_time"`
	RegisterDate  string `json:"register_date"`
	Level         int    `json:"level"`
	Status        int    `json:"status"`
}

// GetManagerList 管理员帐号列表
func (imMgr *ImManager) GetManagerList(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	managers := make([]*managerListResponse, 0)
	rows, err := imMgr.orm.DB().Query("SELECT username, last_login_time, register_date, level, status FROM manager")
	if nil != err {
		c.String(http.StatusInternalServerError, "读取数据库出错")
		return
	}

	for rows.Next() {
		manager := &managerListResponse{}
		if err := rows.Scan(&manager.Username, &manager.LastLoginTime, &manager.RegisterDate, &manager.Level, &manager.Status); nil == err {
			managers = append(managers, manager)
		}
	}

	bytes, err := json.Marshal(managers)
	if nil != err {
		log.Error("获取管理员列表，json序列化出错%v", err)
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}

	c.String(http.StatusOK, string(bytes))
}

type newManagerRequestBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewManager 新建管理员帐号
func (imMgr *ImManager) NewManager(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	requestBody := &newManagerRequestBody{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	if nil != err {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	newManagerUserName := requestBody.Username
	passWord := requestBody.Password
	if "" == newManagerUserName || "" == passWord {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err {
		log.Error("新建管理员帐号,获取帐号失败%v", err)
		c.String(http.StatusInternalServerError, "读取数据出错")
		return
	}

	manager := &models.Manager{}
	bGet, err := imMgr.orm.Where("username = ?", userName).Get(manager)
	if nil != err || !bGet {
		log.Error("新建管理员帐号,获取帐号等级失败%v", err)
		c.String(http.StatusInternalServerError, "读取数据出错")
		return
	}

	level, err := strconv.Atoi(manager.Level)
	if nil != err {
		log.Error("新建管理员帐号,获取帐号等级出错%v", err)
		c.String(http.StatusInternalServerError, "读取数据出错")
		return
	}

	tm := time.Unix(time.Now().Unix(), 0)
	timeStr := tm.Format("2006-01-02 15:04:05")
	insertSql := fmt.Sprintf("INSERT INTO manager (username, password, last_login_time, register_date, level) VALUES ('%s', '%s', '%s', '%s', %s)",
		newManagerUserName, passWord, timeStr, timeStr, strconv.Itoa(level+1))
	_, err = imMgr.orm.DB().Exec(insertSql)
	if nil != err {
		log.Error("新建管理员帐号,插入数据库出错%v", err)
		c.String(http.StatusInternalServerError, "新建帐号失败")
		return
	}

	c.String(http.StatusOK, "新建帐号成功")
}

type SetRegionRegularRequest struct {
	Region  string `json:"region"`
	Regular string `json:"regular"`
}

// SetRegionRegularConfig 设置地区-正则表达式配置
func (imMgr *ImManager) SetRegionRegularConfig(c *gin.Context) {
	/* 校验token和参数 */
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v设置地区-正则表达式配置，获取用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	regionRegularReq := &SetRegionRegularRequest{}
	reqBytes, _ := ioutil.ReadAll(c.Request.Body)
	if err := json.Unmarshal(reqBytes, regionRegularReq); nil != err {
		log.Errorf("用户%s设置地区-正则表达式配置，参数错误%v", userName, err)
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	if "" == regionRegularReq.Region || "" == regionRegularReq.Regular || "+" != regionRegularReq.Region[:1] {
		log.Errorf("用户%s设置地区-正则表达式配置，参数错误%v", userName, err)
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	/* 权限检测 */
	accountAuthority := imMgr.GetManagerAccountAuthority(userName)
	if 0 == CHANGE_CONFIG&accountAuthority {
		log.Errorf("用户%s设置地区-正则表达式配置，权限不足", userName)
		c.String(http.StatusBadRequest, "权限不足")
		return
	}

	/* 设置地区-正则表达式配置 */
	if err := imMgr.rdsRegionRegular.Set(regionRegularReq.Region, regionRegularReq.Regular); nil != err {
		log.Errorf("用户%s设置地区-正则表达式配置撇，设置 redis 失败%v", userName, err)
		c.String(http.StatusInternalServerError, "设置失败")
		return
	}

	c.String(http.StatusOK, "设置成功")
}

type GetRegionRegularConfigResponse struct {
	Region  string `json:"region"`
	Regular string `json:"regular"`
}

// GetRegionRegularConfig 获取地区-正则表达式配置
func (imMgr *ImManager) QueryRegionRegularConfig(c *gin.Context) {
	/* 校验token和参数 */
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v获取地区-正则表达式配置，获取用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	/* 权限检测 */
	accountAuthority := imMgr.GetManagerAccountAuthority(userName)
	if 0 == CHANGE_CONFIG&accountAuthority {
		log.Errorf("用户%s获取地区-正则表达式配置，权限不足", userName)
		c.String(http.StatusBadRequest, "权限不足")
		return
	}

	inters, err := imMgr.rdsRegionRegular.Keys("+*")
	if nil != err {
		log.Errorf("用户%s获取地区-正则表达式配置，获取keys失败%v", userName, err)
		c.String(http.StatusInternalServerError, "获取地区-正则表达式配置失败")
		return
	}

	getRegionRegularConfigResponses := make([]*GetRegionRegularConfigResponse, 0)
	for _, inter := range inters {
		key := string(inter.([]byte))[len(imMgr.rdsRegionRegular.GetPrefix()):]
		regular, _ := imMgr.rdsRegionRegular.Get(key)
		getRegionRegularConfigResponses = append(getRegionRegularConfigResponses, &GetRegionRegularConfigResponse{
			Region:  key,
			Regular: regular,
		})
	}

	bytes, err := json.Marshal(getRegionRegularConfigResponses)
	if nil != err {
		c.String(http.StatusInternalServerError, "json 转换出错")
		return
	}

	c.String(http.StatusOK, string(bytes))
}

// GetManagerAccountAuthority 获取管理员帐号权限
func (imMgr *ImManager) GetManagerAccountAuthority(username string) int64 {
	rows, err := imMgr.orm.DB().Query("SELECT authority FROM manager WHERE username = ?", username)
	if nil != err {
		return 0
	}

	var authority int64
	for rows.Next() {
		if err := rows.Scan(&authority); nil != err {
			return 0
		}
	}

	return authority
}

type onlineUser struct {
	UserId string `json:"user_id"`
}

type onlineUsers struct {
	Count int           `json:"count"`
	Users []*onlineUser `json:"users"`
}

// GetOnlineUser 获取当前在线用户
func (imMgr *ImManager) GetOnlineUser(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	keys, _ := imMgr.rdsUser.Keys("*")
	users := &onlineUsers{
		Count: len(keys),
	}
	for _, key := range keys {
		userIdWithPrefix := key.([]byte)
		userId := strings.TrimPrefix(string(userIdWithPrefix), imMgr.rdsUser.GetPrefix())
		users.Users = append(users.Users, &onlineUser{UserId: userId})
	}

	bytes, err := json.Marshal(users)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		log.Error("用户%v查询在线用户时，json序列化出错%v", token, err)
		return
	}

	c.String(http.StatusOK, string(bytes))
}

type queryLoginAccountRet struct {
	AccountId          int64  `json:"account_id"`
	NickName           string `json:"nick_name"`
	LastLoginTimestamp string `json:"last_login_timestamp"`
	LastLoginIp        string `json:"last_login_ip"`
	DeviceProducter    string `json:"device_producter"`
}

// GetLoginUser 按天查询活跃用户
func (imMgr *ImManager) GetLoginUser(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	beginStr := c.Query("begin")
	endStr := c.Query("end")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")
	if "" == beginStr || "" == endStr || "" == pageStr || "" == pageSizeStr {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	page, err1 := strconv.Atoi(pageStr)
	pageSize, err2 := strconv.Atoi(pageSizeStr)
	if nil != err1 || nil != err2 {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	minLimit := pageSize * (page - 1)
	maxLimit := pageSize
	querySql := fmt.Sprintf("SELECT account_id, nick_name, last_login_timestamp, last_login_ip, device_producter "+
		"FROM account WHERE creation_time>='%s' AND creation_time<='%s' LIMIT %d, %d", beginStr, endStr, minLimit, maxLimit)
	rows, err := imMgr.orm.DB().Query(querySql)
	if nil != err {
		log.Error(fmt.Sprintf("用户%v按天查询活跃用户出错%v", token.Claims, err))
		c.String(http.StatusBadRequest, "检索数据库失败")
	}

	if nil == rows {
		c.String(http.StatusOK, "未检索到结果")
		return
	}

	accounts := make([]queryLoginAccountRet, 0)
	for rows.Next() {
		account := queryLoginAccountRet{}
		if err := rows.Scan(&account.AccountId, &account.NickName, &account.LastLoginTimestamp, &account.LastLoginIp, &account.DeviceProducter); nil == err {
			accounts = append(accounts, account)
		}
	}

	bytes, err := json.Marshal(accounts)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}
	c.String(http.StatusOK, string(bytes))
}

type queryRegisterAccountRet struct {
	AccountId       int64  `json:"account_id"`
	NickName        string `json:"nick_name"`
	CreationTime    string `json:"creation_time"`
	DeviceProducter string `json:"device_producter"`
}

// GetRegisterUser 按天获取注册用户
func (imMgr *ImManager) GetRegisterUser(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	beginStr := c.Query("begin")
	endStr := c.Query("end")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")
	if "" == beginStr || "" == endStr || "" == pageStr || "" == pageSizeStr {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	page, err1 := strconv.Atoi(pageStr)
	pageSize, err2 := strconv.Atoi(pageSizeStr)
	if nil != err1 || nil != err2 {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	minLimit := pageSize * (page - 1)
	maxLimit := pageSize
	querySql := fmt.Sprintf("SELECT account_id, nick_name, creation_time, device_producter FROM account WHERE creation_time>='%s' AND creation_time<='%s' LIMIT %d, %d", beginStr, endStr, minLimit, maxLimit)
	rows, err := imMgr.orm.DB().Query(querySql)
	if nil != err {
		log.Error(fmt.Sprintf("用户%s按天查询注册用户出错%v", token.Claims, err))
		c.String(http.StatusBadRequest, "检索数据库失败")
	}

	if nil == rows {
		c.String(http.StatusOK, "未检索到结果")
		return
	}

	accounts := make([]queryRegisterAccountRet, 0)
	for rows.Next() {
		account := queryRegisterAccountRet{}
		if err := rows.Scan(&account.AccountId, &account.NickName, &account.CreationTime, &account.DeviceProducter); nil == err {
			accounts = append(accounts, account)
		}
	}

	bytes, err := json.Marshal(accounts)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}
	c.String(http.StatusOK, string(bytes))
}

type queryNumRet struct {
	Count int    `json:"count"`
	Date  string `json:"date"`
}

// GetLoginUserNum 按天查询活跃用户数
func (imMgr *ImManager) GetLoginUserNum(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	beginStr := c.Query("begin")
	endStr := c.Query("end")
	querySql := fmt.Sprintf("SELECT COUNT(1),a from "+
		"(SELECT DATE(last_login_timestamp) a FROM account WHERE last_login_timestamp>='%s' AND last_login_timestamp<='%s') ret"+
		" GROUP BY a", beginStr, endStr)
	rows, err := imMgr.orm.DB().Query(querySql)
	if nil != err {
		log.Error(fmt.Sprintf("用户%v按天查询活跃用户数出错%v", token.Claims, err))
		c.String(http.StatusBadRequest, "检索数据库失败")
	}

	if nil == rows {
		c.String(http.StatusOK, "未检索到结果")
		return
	}

	queryRets := make([]queryNumRet, 0)
	for rows.Next() {
		ret := queryNumRet{}
		if err := rows.Scan(&ret.Count, *&ret.Date); nil == err {
			queryRets = append(queryRets, ret)
		}
	}

	bytes, err := json.Marshal(queryRets)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}
	c.String(http.StatusOK, string(bytes))
}

// GetRegisterUserNum 按天获取注册用户数
func (imMgr *ImManager) GetRegisterUserNum(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	beginStr := c.Query("begin")
	endStr := c.Query("end")
	querySql := fmt.Sprintf("SELECT COUNT(1),a from "+
		"(SELECT DATE(creation_time) a FROM account WHERE creation_time>='%s' AND creation_time<='%s') ret"+
		" GROUP BY a", beginStr, endStr)
	rows, err := imMgr.orm.DB().Query(querySql)
	if nil != err {
		log.Error(fmt.Sprintf("用户%v按天查询注册用户数出错%v", token.Claims, err))
		c.String(http.StatusBadRequest, "检索数据库失败")
	}

	if nil == rows {
		c.String(http.StatusOK, "未检索到结果")
		return
	}

	queryRets := make([]queryNumRet, 0)
	for rows.Next() {
		ret := queryNumRet{}
		if err := rows.Scan(&ret.Count, *&ret.Date); nil == err {
			queryRets = append(queryRets, ret)
		}
	}

	bytes, err := json.Marshal(queryRets)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}
	c.String(http.StatusOK, string(bytes))
}

// GetUpdateSet 获取更新设置
func (imMgr *ImManager) GetUpdateSet(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	version := &models.Version{}
	bGet, err := imMgr.orm.Get(version)
	if nil != err || !bGet {
		c.String(http.StatusInternalServerError, "未检索到数据")
		return
	}

	bytes, err := json.Marshal(version)
	if nil != err {
		c.String(http.StatusInternalServerError, "json序列化出错")
		return
	}

	c.String(http.StatusOK, string(bytes))
}

type UpdateSetRequest struct {
	IosMinVersion         string `json:"ios_min_version"`
	IosMaxVersion         string `json:"ios_max_version"`
	IosUpdateUrl          string `json:"ios_update_url"`
	IosUpdateDescribe     string `json:"ios_update_describe"`
	AndroidMinVersion     string `json:"android_min_version"`
	AndroidMaxVersion     string `json:"android_max_version"`
	AndroidUpdateUrl      string `json:"android_update_url"`
	AndroidUpdateDescribe string `json:"android_update_describe"`
}

// UpdateSet 更新设置
func (imMgr *ImManager) UpdateSet(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err {
		log.Error("用户%v更新设置，获取用户名失败%v", userName, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	requestBody := &UpdateSetRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	if nil != err {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	iosMinVersion := requestBody.IosMinVersion
	iosMaxVersion := requestBody.IosMaxVersion
	iosUpdateUrl := requestBody.IosUpdateUrl
	iosUpdateDescribe := requestBody.IosUpdateDescribe
	androidMinVersion := requestBody.AndroidMinVersion
	androidMaxVersion := requestBody.AndroidMaxVersion
	androidUpdateUrl := requestBody.AndroidUpdateUrl
	androidUpdateDescribe := requestBody.AndroidUpdateDescribe
	if "" == iosMinVersion || "" == iosMaxVersion || "" == iosUpdateUrl || "" == iosUpdateDescribe ||
		"" == androidMinVersion || "" == androidMaxVersion || "" == androidUpdateUrl || "" == androidUpdateDescribe {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	_, err = imMgr.orm.Update(&models.Version{
		Id:                    0,
		IosMinVersion:         iosMinVersion,
		IosMaxVersion:         iosMaxVersion,
		IosUpdateUrl:          iosUpdateUrl,
		IosUpdateDescribe:     iosUpdateDescribe,
		AndroidMinVersion:     androidMinVersion,
		AndroidMaxVersion:     androidMaxVersion,
		AndroidUpdateUrl:      androidUpdateUrl,
		AndroidUpdateDescribe: androidUpdateDescribe,
	})
	if nil != err {
		c.String(http.StatusInternalServerError, "更新数据库出错")
		return
	}

	c.String(http.StatusOK, "更新成功")
}

type ChangeAccountStatusRequest struct {
	Account int64 `json:"account"`
	Status  int   `json:"status"`
}

// ChangeAccountStatus 修改帐号状态
func (imMgr *ImManager) ChangeAccountStatus(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err {
		log.Error("用户%v修改帐号状态，获取用户名失败%v", userName, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	requestBody := &ChangeAccountStatusRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	account := requestBody.Account
	status := requestBody.Status
	if 0 != status && 1 != status {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	_, err = imMgr.orm.DB().Exec("UPDATE account SET status = ? WHERE account_id = ?", status, account)
	if nil != err {
		log.Error("管理员修改帐号状态出错%v", err)
		c.String(http.StatusInternalServerError, "修改数据库出错")
		return
	}

	c.String(http.StatusOK, "修改成功")
}

type ResetAccountPasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ResetAccountPassword 修改帐号密码
func (imMgr *ImManager) ResetAccountPassword(c *gin.Context) {
	_, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err {
		log.Error("用户%v修改帐号密码，获取用户名失败%v", userName, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	requestBody := &ResetAccountPasswordRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err = json.Unmarshal(reqBody, requestBody)
	if nil != err || "" == requestBody.Username || "" == requestBody.Password {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	_, err = imMgr.orm.DB().Exec("UPDATE account SET pwd = ? WHERE account_id = %s", requestBody.Password, requestBody.Username)
	if nil != err {
		log.Error("管理员%s重置账户%s密码，修改数据库失败%v", userName, requestBody.Username, err)
		c.String(http.StatusInternalServerError, "修改数据库出错")
		return
	}

	c.String(http.StatusOK, "重置密码成功，新密码为123456")
}

// GetUserLevel 获取管理员帐号等级
func (imMgr *ImManager) GetManagerAccountLevel(username string) (string, error) {
	manager := &models.Manager{}
	_, err := imMgr.orm.Where("username = ?", username).Get(manager)
	if nil != err {
		return "", err
	}

	return manager.Level, nil
}

func (imMgr *ImManager) GetNewThirdPlatformIdDB() (int64, error) {
	rows, err := imMgr.orm.DB().Query("SELECT third_platform_id FROM third_platform WHERE third_platform_id = (SELECT MAX(third_platform_id) FROM third_platform)")
	if nil != err {
		return 0, err
	}
	var newThirdPlatformId int64
	for rows.Next() {
		if err := rows.Scan(&newThirdPlatformId); nil != err {
			return 0, err
		}
	}
	if 0 == newThirdPlatformId {
		newThirdPlatformId = DefaultThirdPlatformId
	} else {
		newThirdPlatformId += 1
	}
	return newThirdPlatformId, nil
}

type RegisterThirdPlatformRequestBody struct {
	ThirdPlatformUsername string `json:"third_platform_username"`
}

var str = "0123456789abcdefghijklmnopqrstuvwxyz"

func GetRandomString(l int) string {
	bytes := []byte(str)
	var result []byte
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

// RegisterThirdPlatform 注册第三方平台
func (imMgr *ImManager) RegisterThirdPlatform(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	registerThirdPlatformRequestBody := &RegisterThirdPlatformRequestBody{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	if err := json.Unmarshal(reqBody, registerThirdPlatformRequestBody); nil != err {
		log.Errorf("用户%v注册第三方平台，读取request body出错%v", token, err)
		c.String(http.StatusInternalServerError, "读取报体出错")
		return
	}
	if "" == registerThirdPlatformRequestBody.ThirdPlatformUsername {
		c.String(http.StatusInternalServerError, "第三方平台名称为空")
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v注册第三方平台,获取 redis 用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	/* 校验帐号权限 */
	lever, err := imMgr.GetManagerAccountLevel(userName)
	if nil != err {
		log.Errorf("用户%s注册第三方平台，获取当前用户等级出错%v或者未获取到", userName, err)
		c.String(http.StatusInternalServerError, "获取当前用户等级失败")
		return
	}

	if "0" != lever {
		log.Errorf("用户%s注册第三方平台，当前账户权限不足，level = %s", userName, lever)
		c.String(http.StatusBadRequest, "当前用户不能注册第三方平台,请与管理员联系")
		return
	}

	/* 注册第三方平台帐号 */
	newThirdPlatformId, err := imMgr.GetNewThirdPlatformIdDB()
	if nil != err {
		log.Errorf("用户%v注册第三方平台,获取新的第三方平台id出错%v", token, err)
		c.String(http.StatusOK, "服务器获取新的第三方平台id出错")
		return
	}
	if _, err := imMgr.orm.Insert(&models.ThirdPlatform{
		ThirdPlatformId:   newThirdPlatformId,
		ThirdPlatformName: registerThirdPlatformRequestBody.ThirdPlatformUsername,
		RegisterTime:      time.Now(),
		AppKey:            GetRandomString(8),
		AppPassword:       strings.Replace(fmt.Sprintf("%v", uuid.NewV4()), "-", "", -1),
	}); nil != err {
		log.Errorf("用户%v注册第三方平台,获取新的第三方平台id出错%v", token, err)
		c.String(http.StatusOK, "服务器注册第三方平台出错")
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("注册成功，平台名称%s", registerThirdPlatformRequestBody.ThirdPlatformUsername))
}

type QueryThirdPlatformResponse struct {
	ThirdPlatformId   int64     `json:"third_platform_id"`
	ThirdPlatformName string    `json:"third_platform_name"`
	AppKey            string    `json:"app_key"`
	AppSecret         string    `json:"app_secret"`
	RegisterTime      time.Time `json:"register_time"`
	Status            int       `json:"status"`
}

// QueryThirdPlatform 查看已注册的第三方平台
func (imMgr *ImManager) QueryThirdPlatform(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v查看第三方平台,获取 redis 用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")
	page, err := strconv.ParseInt(pageStr, 10, 64)
	pageSize, err1 := strconv.ParseInt(pageSizeStr, 10, 64)
	if "" == pageStr || "" == pageSizeStr || nil != err || nil != err1 {
		c.String(http.StatusBadRequest, "page或者pageSize不合法")
		return
	}

	min := pageSize * (page - 1)
	max := pageSize

	queryThirdPlatformResponses := make([]*QueryThirdPlatformResponse, 0)
	rows, err := imMgr.orm.DB().Query("SELECT third_platform_id, third_platform_name, app_key, app_password, register_time, status FROM third_platform LIMIT ?, ?", min, max)
	if nil != err {
		log.Errorf("用户%s查看第三方平台，查询数据库出错%v", userName, err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("查询数据库出错%v", err))
		return
	}

	for rows.Next() {
		response := &QueryThirdPlatformResponse{}
		err := rows.Scan(&response.ThirdPlatformId, &response.ThirdPlatformName, &response.AppKey,
			&response.AppSecret, &response.RegisterTime, &response.Status)
		if nil != err {
			c.String(http.StatusInternalServerError, fmt.Sprintf("查询数据库出错%v", err))
			return
		}
		queryThirdPlatformResponses = append(queryThirdPlatformResponses, response)
	}

	bytes, err := json.Marshal(queryThirdPlatformResponses)
	if nil != err {
		log.Errorf("用户%s查看已注册的第三方平台， json序列化查询已注册的第三方平台出错%v", userName, err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("json序列化查询已注册的第三方平台出错"))
		return
	}

	c.String(http.StatusOK, string(bytes))
}

type ForbidThirdPlatformRequest struct {
	ThirdPlatformId int64 `json:"third_platform_id"`
	Status          int   `json:"status"`
}

// ForbidThirdPlatform 禁用第三方平台
func (imMgr *ImManager) ForbidThirdPlatform(c *gin.Context) {
	token, err := CheckToken(c)
	if nil != err {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	userName, err := imMgr.rdsManager.Get(c.Request.Header.Get("token"))
	if nil != err || 0 == len(userName) {
		log.Error("用户%v禁用第三方平台,获取 redis 用户名失败%v", token, err)
		c.String(http.StatusInternalServerError, "获取用户名失败")
		return
	}

	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	forbidThirdPlatformRequest := &ForbidThirdPlatformRequest{}
	if err := json.Unmarshal(reqBody, forbidThirdPlatformRequest); nil != err {
		log.Errorf("用户%s禁用第三方平台，参数错误%v", userName, err)
		c.String(http.StatusOK, "参数错误")
		return
	}

	if "imusername" != userName {
		log.Errorf("用户%s禁用第三方平台，权限不足", userName)
		c.String(http.StatusBadRequest, "权限不足")
		return
	}

	_, err = imMgr.orm.DB().Exec("UPDATE third_platform SET status = ? WHERE third_platform_id = ?",
		forbidThirdPlatformRequest.Status, forbidThirdPlatformRequest.ThirdPlatformId)
	if nil != err {
		log.Errorf("用户%s禁用第三方平台，修改数据库出错%v", userName, err)
		c.String(http.StatusOK, "修改数据库失败")
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("禁用%d平台成功", forbidThirdPlatformRequest.ThirdPlatformId))
}

// 第三方平台帐号登录IM服务器 获取token 所需参数
type GetPlayerTokenRequest struct {
	UserId    int64  `json:"user_id"`   // 第三方平台帐号id
	UserName  string `json:"user_name"` // 第三方平台帐号username
	AppKey    string `json:"app_key"`   // 第三方平台 appKey
	Timestamp int64  `json:"timestamp"` // 客户端当前时间戳（精确到秒)
	ParentId  int64  `json:"parent_id"` // 第三方平台代理id
	Sign      string `json:"sign"`      // 用户通过签名获取token
}

type GetPlayerTokenResponse struct {
	AccessToken string `json:"access_token"` // token
	ExpiresIn   int    `json:"expires_in"`   // token过期时间
	ErrorCode   int    `json:"error_code"`   // 错误码 0：表示成功
	ErrorMsg    string `json:"error_msg"`    // 错误消息
}

func (imMgr *ImManager) GetPlayerTokenResponse(token, errorMsg string, expiresIn, errorCode int) string {
	bytes, _ := json.Marshal(&GetPlayerTokenResponse{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		ErrorCode:   errorCode,
		ErrorMsg:    errorMsg,
	})
	return string(bytes)
}

// GetPlayerToken
func (imMgr *ImManager) GetPlayerToken(c *gin.Context) {
	request := &GetPlayerTokenRequest{}
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	err := json.Unmarshal(reqBody, request)
	if nil != err {
		log.Errorf("第三方平台玩家帐号登录请求token，参数错误%v", err)
		c.String(http.StatusBadRequest, imMgr.GetPlayerTokenResponse("", "参数错误", 0, 1))
		return
	}

	/* 获取第三方平台id */
	platform := &models.ThirdPlatform{}
	bGet, err := imMgr.orm.Where("app_key = ?", request.AppKey).Get(platform)
	if nil != err || !bGet {
		log.Errorf("第三方平台玩家帐号登录请求token，第三方平台不存在%v", err)
		c.String(http.StatusBadRequest, imMgr.GetPlayerTokenResponse("", "第三方平台未注册", 0, 2))
		return
	}

	if 0 != request.ParentId {
		/* 校验上级代理是否存在 */
		rows, err := imMgr.orm.DB().Query("SELECT account_id FROM account WHERE account_id = ?", request.ParentId+platform.ThirdPlatformId*DefaultThirdPlatformUserId)
		if nil != err {
			log.Errorf("第三方平台玩家帐号登录请求token，代理不存在%v", err)
			c.String(http.StatusBadRequest, imMgr.GetPlayerTokenResponse("", "代理不存在", 0, 3))
			return
		}
		var parentId int64 = 202001151553
		for rows.Next() {
			rows.Scan(&parentId)
		}
		if request.ParentId+platform.ThirdPlatformId*DefaultThirdPlatformUserId != parentId {
			log.Errorf("第三方平台玩家帐号登录请求token，代理不存在%v", err)
			c.String(http.StatusBadRequest, imMgr.GetPlayerTokenResponse("", "代理不存在", 0, 3))
			return
		}
	}

	// /* 校验签名 */
	// if !CheckSign(request, platform.AppPassword) {
	// 	log.Errorf("第三方平台玩家帐号登录请求token，签名校验失败")
	// 	c.String(http.StatusBadRequest, imMgr.GetPlayerTokenResponse("", "签名校验失败", 0, 4))
	// 	return
	// }
	request.UserId += platform.ThirdPlatformId * DefaultThirdPlatformUserId

	rows, err := imMgr.orm.DB().Query("SELECT account_id, pwd FROM account WHERE account_id = ?", request.UserId)
	if nil != err {
		log.Errorf("第三方平台玩家帐号登录请求token，查询用户帐号%d, %s失败", request.UserId, request.UserName)
		c.String(http.StatusInternalServerError, imMgr.GetPlayerTokenResponse("", "查询用户帐号失败", 0, 5))
		return
	}
	var accountId int64 = 202001151553
	pwd := function.Md5_32bit(DefaultAgentPassword)
	for rows.Next() {
		rows.Scan(&accountId, pwd)
	}

	if request.UserId != accountId {
		rdsSessReg := redisObj.NewSessionWithPrefix("registerstatus")
		// 用分布式锁来保障同一时刻只有一个同名用户登录
		_ = rdsSessReg.Setex(strconv.FormatInt(request.UserId, 10), time.Second*30, 1)
		defer rdsSessReg.Del(strconv.FormatInt(request.UserId, 10))

		nickName := GetRandomString(10)
		if nil != imMgr.CreateThirdPlatformAccountDB(request, platform.ThirdPlatformId, nickName, pwd) {
			log.Errorf("第三方平台玩家帐号登录请求token，创建用户帐号%d, %s失败", request.UserId, request.UserName)
			c.String(http.StatusInternalServerError, imMgr.GetPlayerTokenResponse("", "创建用户帐号失败", 0, 5))
			return
		}
		if bytes, b := function.ProtoMarshal(&msg_struct.UserBriefInfo{
			AccountId:    request.UserId,
			Nickname:     nickName,
			UserName:     request.UserName,
			HeadPortrait: "",
			Signature:    "",
		}, "msg_struct.UserBriefInfo"); b {
			imMgr.rdsBriefInfo.Set(strconv.FormatInt(request.UserId, 10), string(bytes))
		} else {
			log.Errorf("插入 redis briefinfo 失败")
		}
	}

	loginToken, err := CreateToken([]byte(tokenKey), request.UserName, request.UserId, time.Now().Add(time.Minute*10).Unix(), snowflake.GetSnowflakeId())
	if nil != err {
		log.Errorf("第三方平台玩家帐号登录请求token，生成token出错")
		c.String(http.StatusInternalServerError, imMgr.GetPlayerTokenResponse("", "生成token出错", 0, 5))
		return
	}
	_ = imMgr.rdsLoginToken.Setex(loginToken, time.Minute*10, strconv.FormatInt(request.UserId, 10)+"-"+pwd)

	if 0 != request.ParentId {
		_, _ = imMgr.rdsAgent.AddSetMembers(strconv.FormatInt(request.ParentId+platform.ThirdPlatformId*DefaultThirdPlatformUserId, 10),
			strconv.FormatInt(request.UserId, 10))
	}

	c.String(http.StatusOK, imMgr.GetPlayerTokenResponse(loginToken, "", 600, 0))
}

func CheckSign(req *GetPlayerTokenRequest, password string) bool {
	return req.Sign == strings.ToUpper(function.Md5_32bit(
		fmt.Sprintf("app_key=%s&parent_id=%d&timestamp=%d&user_id=%d&user_name=%s",
			req.AppKey, req.ParentId, req.Timestamp, req.UserId, req.UserName)+password))
}

func (imMgr *ImManager) CreateThirdPlatformAccountDB(req *GetPlayerTokenRequest, thirdPlatformId int64, nickname, pwd string) error {
	// 事务开始
	tx := imMgr.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("第三方平台玩家帐号登录请求token，开始数据库事务失败,error:%v", err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("第三方平台玩家帐号登录请求token, 回滚事务 tx.Rollback() 出现严重错误: %v", e)
				}
			}
		}
	}()

	var agentId int64
	if 0 != req.ParentId {
		agentId = req.ParentId + thirdPlatformId*DefaultThirdPlatformUserId
	}
	// 事务step1 插入account表
	if _, err = tx.Insert(&models.Account{
		AccountId:    req.UserId,
		NickName:     nickname,
		UserName:     req.UserName,
		Pwd:          pwd,
		AgentId:      agentId,
		CreationTime: time.Now(),
	}); nil != err {
		errCode = 1
		log.Errorf("第三方平台玩家帐号登录请求token，插入account表失败%v", err)
		return err
	}

	if 0 != req.ParentId {
		// 事务step2 插入agent_client表
		if _, err = tx.Insert(&models.AgentClient{
			ClientId:        req.UserId,
			AgentId:         req.ParentId + thirdPlatformId*DefaultThirdPlatformUserId,
			ThirdPlatformId: thirdPlatformId,
		}); nil != err {
			errCode = 2
			log.Errorf("第三方平台玩家帐号登录请求token，插入agent_client表失败%v", err)
			return err
		}
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("第三方平台玩家帐号登录请求token, 提交事务 tx.Commit() 出现严重错误: %v", err)
		return err
	}

	return nil
}

func main() {
	imPublicConfig, err := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err {
		log.Errorf("读取配置 public.json 文件出错:%v", err)
		return
	}

	redisObj.Init(imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd)

	imManager := NewImManager(imPublicConfig.Mysql)
	panic(imManager.httpSrv.ListenAndServe())
}
