package client

import (
	"encoding/json"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"strconv"
)

func init() {
	registerLogicCall(msg_id.NetMsgId_C2SRename, C2SRename)
}

func C2SRename(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SRename{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SRename") {
		if c.GetName() != reqMsg.Newname && reqMsg.Newname != "" {
			c.SetName(reqMsg.Newname)
			c.SaveUsrBriefInfoToRedis()
			c.rdsUserNameInfo.Set(c.GetStrAccount(), reqMsg.Newname)

			strAccountId := c.GetStrAccount()
			myChatGroupIds := c.GetChatGroupIds()

			for _, chatGroupId := range myChatGroupIds {
				usrChatGroupInfo, e := c.rdsGroupChatRecord.GetHashSetField("hash:"+strconv.FormatInt(chatGroupId, 10), strAccountId)
				if e != nil {
					log.Errorf("account: %d 改名时,查询 redis 中的群( %d )成员信息出错", c.GetAccountId(), chatGroupId)
					continue
				}

				groupMemberRds := &msg_struct.ChatGroupMemberRds{}
				e = json.Unmarshal([]byte(usrChatGroupInfo), groupMemberRds)
				if nil != e {
					log.Errorf("account: %d 改名时,查询出 redis 中的群( %d )成员信息后, 用 json 反序列化是出错 %v", c.GetAccountId(), chatGroupId, e)
					continue
				}
				groupMemberRds.UserNickName = reqMsg.Newname
				bytes, e2 := json.Marshal(groupMemberRds)
				if e2 != nil {
					log.Errorf("account: %d 改名时,用 json 序列化是出错 %v", c.GetAccountId(), e2)
					continue

				}

				c.rdsGroupChatRecord.HashSet("hash:"+strconv.FormatInt(chatGroupId, 10), strAccountId, bytes)
			}
		}

		ackMsg := &msg_struct.S2CRename{Newname: reqMsg.Newname}
		c.SendPbMsg(msg_id.NetMsgId_S2CRename, ackMsg)
	}
}
