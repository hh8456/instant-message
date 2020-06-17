package main

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/gomodule/redigo/redis"
	"github.com/hh8456/go-common/redisObj"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/msg-server/client"
	"os"
	"strconv"
	"time"
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

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/syncHuanxinAccount", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func main() {
	/* 初始化xorm和redis */
	imPublicConfig, err := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err {
		log.Errorf("读取配置 public.json 文件出错:%v", err)
		return
	}
	redisObj.Init(imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd)
	rdsHuanXin := redisObj.NewSessionWithPrefix(client.RdsKeyPrefixHuanXinString)
	log.Tracef("初始化redis成功，ip：%s,password:%s,prefixKey:%s", imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd, client.RdsKeyPrefixHuanXinString)

	engine, err := xorm.NewEngine("mysql", imPublicConfig.Mysql)
	if nil != err {
		log.Error("初始化mysql失败%v, config:%s", err, imPublicConfig.Mysql)
		return
	}
	log.Tracef("初始化mysql成功,config:%s", imPublicConfig.Mysql)

	rows, err := engine.DB().Query("SELECT account_id, nick_name FROM account")
	if nil != err {
		log.Error("从数据库读取account帐号，昵称失败%v", err)
		return
	}
	defer rows.Close()
	log.Tracef("从数据库读取account帐号，昵称成功")

	rdsHuanXin.Del("")
	huanXinToken, err := rdsHuanXin.Get("token")
	if nil != err {
		if redis.ErrNil == err {
			log.Warnf("从 redis 获取环信 token 失败: 环信 token 已过期")
		} else {
			log.Errorf("从 redis 获取环信 token 失败:%v, 重置 token", err)
			return
		}

		response := client.GetHuanXinToken()
		_, _ = rdsHuanXin.Del("token")
		err := rdsHuanXin.Setex("token", time.Duration(response.ExpiresIn)*time.Second, response.AccessToken)
		if nil != err {
			log.Errorf("设置环信 token 存入 redis 失败:%v", err) // 此处不返回空，毕竟token在这一次是可用的
			return
		}
	}
	log.Tracef("从 redis 获取环信 token 成功")

	for rows.Next() {
		var (
			accountId int64
			nickName  string
		)
		if err := rows.Scan(&accountId, &nickName); nil != err {
			log.Errorf("从数据库读取accountId，nickName失败")
			return
		}
		userName := strconv.FormatInt(accountId, 10)
		huanXinNewPassword := fmt.Sprintf("%d%d", accountId, time.Now().Unix())
		_, err := client.CreateHuanXinAccount(huanXinToken, userName, huanXinNewPassword, nickName)
		if nil != err {
			log.Errorf("用户%d注册环信账户时，注册失败：%v", accountId, err)
			return
		}
		log.Tracef("为帐号%d，昵称%s创建环信帐号成功，密码:%s", accountId, userName, huanXinNewPassword)

		if err := rdsHuanXin.Set(userName, huanXinNewPassword); nil != err {
			log.Errorf("用户%d注册环信账户时，注册成功，存入 redis 失败：%v", accountId, err)
			return
		}
		log.Tracef("为帐号%d，昵称%s创建环信帐号，密码%s，并存入redis成功", accountId, userName, huanXinNewPassword)
	}
}
