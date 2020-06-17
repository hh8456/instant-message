package main

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/hh8456/go-common/redisObj"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_struct"
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

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/syncMysqlContactList2Redis", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func main() {
	/* 初始化xorm和redis */
	imPublicConfig, err1 := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err1 {
		log.Errorf("读取配置 public.json 文件出错:%v", err1)
		return
	}
	redisObj.Init(imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd)
	log.Tracef("初始化redis成功，ip：%s,password:%s", imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd)

	orm := function.Must(xorm.NewEngine("mysql", imPublicConfig.Mysql)).(*xorm.Engine)
	orm.SetMaxIdleConns(5)
	orm.SetMaxOpenConns(10)
	orm.SetConnMaxLifetime(time.Minute * 14)
	log.Tracef("初始化mysql-xorm成功：%s", imPublicConfig.Mysql)

	redisSession := redisObj.NewSessionWithPrefix(client.RdsKeyPrefixFriendSet)
	log.Tracef("初始化redisSession成功，key:%s", client.RdsKeyPrefixFriendSet)

	rows, err := orm.DB().Query("SELECT account_id, contact_list FROM account")
	if nil != err {
		log.Errorf("从数据库读取account表，出错：%v", err)
		panic(err)
	}
	defer rows.Close()
	log.Tracef("读取数据库所有account数据成功")

	for rows.Next() {
		var (
			accountId        int64
			contactListsByte []byte
		)
		if err := rows.Scan(&accountId, &contactListsByte); nil == err {
			if nil == contactListsByte {
				log.Tracef("用户%d的contact_list为空", accountId)
				continue
			}
			contactLists := &msg_struct.ContactList{}
			if !function.ProtoUnmarshal(contactListsByte, contactLists, "msg_struct.ContactList") {
				log.Errorf("用户%d的contact_list反序列化为msg_struct.ContactList出错")
				continue
			}
			for _, contact := range contactLists.Contacts {
				fmt.Printf("读取到用户%d的好友Account:%d", accountId, contact.AccountId)
				addSetMembers, err := redisSession.AddSetMembers(strconv.FormatInt(accountId, 10), contact.AccountId)
				if nil == err || 1 == addSetMembers {
					log.Tracef("用户%d的好友Account：%d插入redis成功", accountId, contact.AccountId)
				} else {
					log.Errorf("用户%d的好友Account：%d插入redis失败%v", accountId, contact.AccountId, err)
				}
			}
		} else {
			log.Errorf("用户%d的contact_list读取scan()失败:%v", err)
			panic(err)
		}
	}
}
