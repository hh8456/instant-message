package login_app

import (
	"encoding/binary"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
)

func (l *loginApp) listenClientConnection(addr string) {

	defer function.Catch()

	listenerPtr := im_net.CreateListener("tcp", addr, func(conn net.Conn) {
		c := im_net.CreateSocket(conn, 1<<14)
		go l.receiveClientConnection(c)
	})

	for {
		err := listenerPtr.Start()
		if err != nil {
			log.Errorf("listen client connection error:%s", err.Error())
		}

		time.Sleep(time.Second)
	}
}

func (l *loginApp) receiveClientConnection(c *im_net.Socket) {
	defer function.Catch()
	b := false
	for {
		bf, err := c.ReadOne()
		if err != nil {
			if err != io.EOF {
				log.Errorf("receive client[%v] login msg error:%s", c.RemoteAddr(), err.Error())
			}
			c.Close()
			return
		}

		if len(bf) < 4 {
			log.Errorf("receive client login msg error,getting pid error,buffer len:%d", len(bf))
			c.Close()
			return
		}

		// 登录服务器只接受客户端一次请求
		if b {
			log.Errorf("发现有个客户端登录时发送 2 次请求, 应该只发送一次;关闭它的连接")
			c.Close()
			return
		}

		pid := binary.BigEndian.Uint32(bf)

		switch pid {
		case uint32(msg_id.NetMsgId_C2l_gate_addr):
			b = true
			l.loginLogic(bf[packet.PacketHeaderSize:], c)

		default:
			log.Errorf("receive client pid error:%d", pid)
			c.Close()
			return
		}
	}
}

func (l *loginApp) loginLogic(bf []byte, socket *im_net.Socket) {
	var err error
	reqPtr := &msg_struct.C2LGateAddr{}
	resPtr := &msg_struct.L2CGateAddr{}

	err = proto.Unmarshal(bf, reqPtr)
	if err != nil {
		log.Errorf("unmarshal c2l gate addr error:%s", err.Error())
		p := &msg_struct.S2CErrorCode{}
		p.ErrCode = int64(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SLogin_failed)
		socket.SendPbMsg(msg_id.NetMsgId_S2CErrorCode, p)
		return
	}

	accountId := reqPtr.GetAccountId()
	if accountId == 0 {
		log.Errorf("c2l gate addr accountId is 0")
		p := &msg_struct.S2CErrorCode{}
		p.ErrCode = int64(msg_err_code.MsgErrCode_login_account_id_is_zero)
		socket.SendPbMsg(msg_id.NetMsgId_S2CErrorCode, p)
		return
	}

	resPtr.Ip = l.hashRing.GetNode(strconv.Itoa(int(accountId)))
	// XXX 最小连接数负载均衡算法
	//resPtr.Ip, _ = l.onlineUserNumber.GetMininumLoad()
	socket.SendPbMsg(msg_id.NetMsgId_L2c_gate_addr, resPtr)
}
