package main

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"os"
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

// 如果需要修改表名，请使用替换，而不要单独改表名
const (
	// MysqlAccountTable 帐号表
	MysqlAccountTable string = "account"

	// MysqlFriendReqTable 申请加好友表
	MysqlFriendReqTable string = "friend_req"
	// MysqlFriendVerificationTable 通过加好友申请表
	MysqlFriendVerificationTable string = "friend_verification"
	// MysqlFriendDeleteTable 好友关系解除表
	MysqlFriendDeleteTable string = "friend_delete"

	// MysqlPrivateChatRecordTable 私聊记录表
	MysqlPrivateChatRecordTable string = "private_chat_record"

	// MysqlChatGroupTable 聊天群表
	MysqlChatGroupTable string = "chat_group"
	// MysqlChatGroupCreateLogTable 聊天群创建记录表
	MysqlChatGroupCreateLogTable string = "chat_group_create_log"
	// MysqlChatGroupCreateLogTable 聊天群注销记录表
	MysqlChatGroupDestroyLogTable string = "chat_group_destroy_log"
	// MysqlChatGroupMemberTable 聊天群群成员列表表
	MysqlChatGroupMemberTable string = "chat_group_member"
	// MysqlChatGroupMemberQuitLogTable 聊天群群成员退群记录表
	MysqlChatGroupMemberQuitLogTable string = "chat_group_member_quit_log"
	// MysqlChatGroupReqJoinLogTable 聊天群请求加入记录表
	MysqlChatGroupReqJoinLogTable string = "chat_group_req_join_log"

	// MysqlWeiboTable 朋友圈表
	MysqlWeiboTable string = "weibo"
	// MysqlWeiboCoverTable 朋友圈封面表
	MysqlWeiboCoverTable string = "weibo_cover"
	// MysqlWeiboCoverDeleteTable 朋友圈封面更换表
	MysqlWeiboCoverDeleteTable string = "weibo_cover_delete_log"
	// MysqlWeiboDeleteTable 朋友圈删除记录表
	MysqlWeiboDeleteTable string = "weibo_deleted"
	// MysqlWeiboLikeTable 朋友圈点赞表
	MysqlWeiboLikeTable string = "weibo_like"
	// MysqlWeiboReplyTable 朋友圈评论表
	MysqlWeiboReplyTable string = "weibo_reply"

	// MysqlThirdPlatformTable 第三方平台表
	MysqlThirdPlatformTable string = "third_platform"
	// MysqlThirdPlatformAgent 第三方平台与代理关系表
	MysqlThirdPlatformAgent string = "third_platform_agent"
	// MysqlAgentClientTable 第三方平台代理帐号关系表
	MysqlAgentClientTable string = "agent_client"

	// MysqlVersionTable IM项目版本表
	MysqlVersionTable string = "version"
)

const (
	// MysqlCheckAccountTable 帐号表建表sql
	MysqlCheckAccountTable string = `
		CREATE TABLE IF NOT EXISTS account1 (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '自增ID，不参与代码逻辑',
		account_id BIGINT NOT NULL COMMENT '帐号id(一般为电话号码)',
		user_name VARCHAR(128) DEFAULT NULL COMMENT '用户名(只能设置一次)',
		nick_name VARCHAR(128) NOT NULL COMMENT '昵称',
		contact_list BLOB COMMENT '联系人列表',
		black_list BLOB COMMENT '黑名单列表',
		last_msg_timestamp BIGINT NOT NULL COMMENT '收到的最后一条消息的时间戳' DEFAULT 0,
		creation_time DATETIME NOT NULL COMMENT '角色创建时间',
		pwd VARCHAR(128) NOT NULL COMMENT '密码',
		head_portrait VARCHAR(128) COMMENT '头像url地址',
		last_login_timestamp BIGINT UNSIGNED COMMENT '上次登录时间',
		last_login_ip VARCHAR(128) COMMENT '客户端上次登录ip地址',
		device_producer VARCHAR(128) COMMENT '设备厂商(app手机厂商)',
		device_info VARCHAR(128) COMMENT '手机推送ID',
		status INT(2) NOT NULL DEFAULT 0 COMMENT '帐号状态，0为启用，1为禁用',
		agent_id BIGINT COMMENT '用户的上级代理id; 如果没有就填 0',
		UNIQUE KEY (id),
		UNIQUE KEY (account_id),
		UNIQUE KEY (user_name),
		KEY (account_id, pwd),
		KEY (user_name, pwd)
		)
		COMMENT '帐号表'
		ENGINE=InnoDB
		DEFAULT CHARACTER SET=utf8mb4 COLLATE=utf8mb4_general_ci
		AUTO_INCREMENT=1;
	`
)

func main() {
	/* 读取配置文件 */
	imPublicConfig, err := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err {
		log.Errorf("读取配置 public.json 文件出错:%v", err)
		return
	}

	orm := function.Must(xorm.NewEngine("mysql", imPublicConfig.Mysql)).(*xorm.Engine)
	orm.SetMaxIdleConns(5)
	orm.SetMaxOpenConns(10)
	orm.SetConnMaxLifetime(time.Minute * 14)
	log.Tracef("初始化mysql-xorm成功：%s", imPublicConfig.Mysql)

	createAccountTable(orm)
}

func createAccountTable(orm *xorm.Engine) {
	_, err := orm.DB().Exec(MysqlCheckAccountTable)
	if nil != err {
		log.Errorf("创建%s失败", MysqlAccountTable)
		return
	}
	log.Tracef("创建%s表成功", MysqlAccountTable)
}
