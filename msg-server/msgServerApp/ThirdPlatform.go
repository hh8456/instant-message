package msgServerApp

import (
	"database/sql"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"instant-message/msg-server/client"
	"instant-message/msg-server/clientManager"
	"instant-message/msg-server/eventmanager"
	"net"
	"strconv"
	"strings"
	"time"
)

// ThirdPlatformLogin 第三方平台帐号登录
func (g *msgServerObj) ThirdPlatformLogin(conn net.Conn, c *im_net.Socket, buf []byte) {
	accountLogin := &msg_struct.C2SThirdPlatformAccountLogin{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], accountLogin, "msg_struct.C2SThirdPlatformAccountLogin") {
		return
	}

	strAccountIdAndPwd, err := g.rdsLoginToken.Get(accountLogin.Token)
	defer g.rdsLoginToken.Del(accountLogin.Token)
	if nil != err {
		return
	}
	splits := strings.Split(strAccountIdAndPwd, "-")
	strAccountId := splits[0]
	pwd := splits[1]
	accountId, _ := strconv.ParseInt(strAccountId, 10, 64)

	// 去数据库中查找
	account, e1 := g.queryAccountById(accountId, pwd)
	if e1 != nil {
		if e1 == sql.ErrNoRows {
			sendErrCode(c, msg_err_code.MsgErrCode_login_account_or_pwd_is_wrong, 0)
			log.Errorf("用户名或者密码错误, account_id: %d", accountId)
			c.Close()
			return
		} else {
			log.Errorf("客户端登录时, accountId:%d, 读取 mysql 出错: %v", accountId, e1)
			sendErrCode(c, msg_err_code.MsgErrCode_login_access_mysql_has_error_when_load_account, 0)
			c.Close()
			return
		}
	}

	/* 判断帐号状态，1为停用，0为启用 */
	if account.Status == 1 {
		log.Tracef("客户端%d登录，账号已停用")
		sendErrCode(c, msg_err_code.MsgErrCode_login_account_is_out_of_service, 0)
		return
	}

	if account == nil {
		log.Errorf("客户端登录, 不应该出现这条日志")
		c.Close()
		return
	}

	// 用分布式锁来保障同一时刻只有一个同名用户登录
	e2 := g.rdsLogin.Setex(strAccountId, time.Second*30, 1)
	if e2 != nil {
		c.Close()
		return
	}

	defer g.rdsLogin.Del(strAccountId)

	// 处理顶号
	done := clientManager.KickClient(accountId, accountLogin.TerminalType)
	<-done

	cli := client.NewClient(account, conn, c, g.orm, accountLogin.TerminalType, g.chatRecord)
	// 把简略信息保存到 redis
	cli.SaveUsrBriefInfoToRedis()
	clientManager.StoreClient(cli)
	log.Tracef("account: %d 登录后,保存 client 对象到内存中", accountId)
	clientManager.WatchCloseSignal(cli)

	cli.GetAgentAndMember()

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", cli)
	eventmanager.Publish(eventmanager.ClientLoginEvent, eventArg)

	// 处理登录后, 只管转发消息
	cli.Run()

	log.Tracef("account: %d 登录", cli.GetAccountId())

	client.SetClientOnlineExpire(cli, cli.GetAccountId())

	g.recordLogin(cli.GetAccountId(), c)

	cli.SendPbMsg(msg_id.NetMsgId_S2CThirdPlatformAccountLogin, &msg_struct.S2CThirdPlatformAccountLogin{
		AccountId:        accountId,
		Nickname:         account.UserName,
		LastMsgTimestamp: account.LastMsgTimestamp,
		AgentId:          account.AgentId,
		AgentNickname:    cli.GetAgentOrMemberUsername(account.AgentId),
	})
}
