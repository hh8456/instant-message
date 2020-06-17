package msgServerApp

import (
	"encoding/binary"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/msg-server/clientManager"
	"instant-message/msg-server/onlineChatGroupManager"
)

// XXX 把 pulsar 替换为 redis, 系统稳定后删除注释 19.10.20
//func (g *msgServerObj) recvFromPulsar(topics []string) {
//for _, v := range topics {
//c := pulsar_client.GetMsgFromTopic(v)

//if c != nil {
//go func() {
//for {
//pulsarMsg, ok := <-c
//if !ok {
//return
//}

//if len(pulsarMsg.Payload()) >= 24 { // 前面 16 个字节 + 8个字节的消息头
//g.handlePulsarMsg(pulsarMsg.Payload())
//}
//}
//}()
//}
//}
//}

//func (g *msgServerObj) handlePulsarMsg(buf []byte) {
//receiverId := int64(binary.BigEndian.Uint64(buf))
//groupId := int64(binary.BigEndian.Uint64(buf[8:]))
//if receiverId > 0 {
//receiver := clientManager.GetClient(receiverId)
//if receiver != nil {
//receiver.HandleMsg(buf[16:])
//}

//return
//}

//if groupId > 0 {
//msgId := binary.BigEndian.Uint32(buf[16:])
//switch msgId {
//// 发送群消息
//case uint32(msg_id.NetMsgId_PulsarGroupChat):
//g.handleSendGroupChat(buf[16:])
//}
//}
//}

func (g *msgServerObj) handleSendGroupChat(msg []byte) {
	p := &msg_struct.GroupMessage{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:],
		p,
		"msg_struct.GroupMessage") {
		onlineChatGroupManager.SendToLocalChatGroup(p.ReceiveGroupId,
			msg_id.NetMsgId_S2CRecvGroupChat, p)
	}
}

func (g *msgServerObj) HandleMQMessage(buf []byte) {
	if len(buf) >= 24 { // 前面 16 个字节 + 8个字节的消息头
		// 0-7   字节是消息接收者 id
		// 8-15  字节是消息接收群 id
		// 16-19 字节是消息 id
		// 20-23 字节是消息长度
		// 24 字节开始,是 proto 二进制数据
		receiverId := int64(binary.BigEndian.Uint64(buf))
		log.Tracef("receiverId = %d", receiverId)
		groupId := int64(binary.BigEndian.Uint64(buf[8:]))
		if receiverId > 0 {
			receiver := clientManager.GetClient(receiverId, clientManager.TerminalTypeApp)
			if receiver != nil {
				receiver.HandleMsg(buf[16:])
			} else {
				log.Tracef("MQ 转发消息时, 接收者 %d 的 app 端不在线", receiverId)
			}

			receiver = clientManager.GetClient(receiverId, clientManager.TerminalTypeWeb)
			if receiver != nil {
				receiver.HandleMsg(buf[16:])
			} else {
				log.Tracef("MQ 转发消息时, 接受者 %d 的 web 端不在线", receiverId)
			}

			return
		}

		if groupId > 0 {
			msgId := binary.BigEndian.Uint32(buf[16:])
			switch msgId {
			// 发送群消息
			case uint32(msg_id.NetMsgId_MQGroupChat):
				g.handleSendGroupChat(buf[16:])
			}
		}
	}
}
