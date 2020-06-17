package clientManager

import (
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/msg-server/client"
	"instant-message/msg-server/eventmanager"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
)

const (
	TerminalTypeApp int32 = 1 // APP 客户端
	TerminalTypeWeb int32 = 2 // web 客户端
)

var (
	lock sync.RWMutex
	//mapOnlineClient map[int64]*client.Client //  accountId( 在线用户 ) - *client.Client
	mapOnlineClient map[int64]map[int32]*client.Client //  accountId( 在线用户 ) - *client.Client
)

func init() {
	mapOnlineClient = map[int64]map[int32]*client.Client{}
}

// //IsClientOnline 判断用户是否在线
//func IsClientOnline(id int64) *client.Client {
//lock.RLock()
//defer lock.RUnlock()
//return mapOnlineClient[id]
//}

func StoreClient(cli *client.Client) {
	terminalType := cli.GetTerminalType()
	lock.Lock()
	mapClient, ok := mapOnlineClient[cli.GetAccountId()]
	if ok {
		mapClient[terminalType] = cli
	} else {
		m := map[int32]*client.Client{}
		m[terminalType] = cli
		mapOnlineClient[cli.GetAccountId()] = m
	}

	//mapOnlineClient[cli.GetAccountId()] = cli
	lock.Unlock()
}

func KickClient(accountId int64, terminalType int32) chan interface{} {
	lock.RLock()
	defer lock.RUnlock()
	if mapClient, exits := mapOnlineClient[accountId]; exits {
		if cli, ok := mapClient[terminalType]; ok {
			done := cli.GetDoneSignal()
			go func() {
				cli.BlockingSendPbMsg(msg_id.NetMsgId_S2CLoginSomewhereElse,
					&msg_struct.S2CLoginSomewhereElse{SrvTimeStamp: time.Now().UnixNano() / 1e3})
				switch terminalType {
				case TerminalTypeApp:
					log.Tracef("account: %d 手机 APP 端顶号, ", accountId)

				case TerminalTypeWeb:
					log.Tracef("account: %d WEB 端顶号, ", accountId)

				default:
					log.Errorf("account: %d 顶号, 但无法判断是 APP 端还是 web 端", accountId)
				}

				// cli.Close() 会触发 clientManager.WatchCloseSignal 函数
				// clientManager.WatchCloseSignal 函数会触发 RemoveClient
				// RemoveClient 函数会触发 client.Done()
				cli.Close()
			}()

			return done
		}
	}

	done := make(chan interface{})
	go func() {
		close(done)
	}()

	return done
}

func RemoveClient(cli *client.Client) {
	lock.Lock()
	if mapClient, exits := mapOnlineClient[cli.GetAccountId()]; exits {
		if _, find := mapClient[cli.GetTerminalType()]; find {
			delete(mapClient, cli.GetTerminalType())
			if len(mapClient) == 0 {
				delete(mapOnlineClient, cli.GetAccountId())
			}
		}
	}
	lock.Unlock()

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", cli)
	eventmanager.Publish(eventmanager.ClientLogoutEvent, eventArg)

	cli.Done()
}

func GetClient(accountId int64, terminalType int32) *client.Client {
	lock.RLock()
	mapClient, find := mapOnlineClient[accountId]
	if find {
		cli, ok := mapClient[terminalType]
		if ok {
			lock.RUnlock()
			return cli
		}
	}

	lock.RUnlock()
	return nil
}

func WatchCloseSignal(cli *client.Client) {
	go func() {
		closeSignal := cli.GetCloseSignal()
		<-closeSignal

		RemoveClient(cli)
	}()
}

func GetOnlineUserNumber() int {
	lock.RLock()
	defer lock.RUnlock()
	return len(mapOnlineClient)
}

// 获得在另外一种终端上登录的 client
func GetOtherTerminalClient(cli *client.Client) *client.Client {
	var otherTerminalClient *client.Client
	if cli.GetTerminalType() == TerminalTypeApp {
		otherTerminalClient = GetClient(cli.GetAccountId(), TerminalTypeWeb)
	} else {
		otherTerminalClient = GetClient(cli.GetAccountId(), TerminalTypeApp)
	}

	return otherTerminalClient
}

func SendPbMsgToOtherTerminal(cli *client.Client, cmdId msg_id.NetMsgId, pb proto.Message) {
	otherTerminalClient := GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		otherTerminalClient.SendPbMsg(cmdId, pb)
	}
}
