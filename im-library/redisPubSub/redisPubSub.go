package redisPubSub

import (
	"encoding/binary"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/im-library/consistentHash"
	"strconv"

	"github.com/g4zhuj/hashring"
	"github.com/gogo/protobuf/proto"
	"github.com/hh8456/go-common/redisObj"
	"github.com/hh8456/redisSession"
)

var (
	rdsSessPubSub *redisSession.RedisSession // 用于消息订阅发布
	hashRing      *hashring.HashRing
)

func Init(ipAndWeights []string) {
	rdsSessPubSub = redisObj.NewSessionWithPrefix("pub") // rdsSessPubSub 的名字前缀没有任何意义
	hashRing = consistentHash.New(ipAndWeights)
}

func SendPbMsg(receiverId int64, cmdId msg_id.NetMsgId, pb proto.Message) error {
	msg, err := proto.Marshal(pb)
	if err == nil {
		IpAndPort := hashRing.GetNode(strconv.Itoa(int(receiverId)))
		buf := make([]byte, 16+packet.PacketHeaderSize+len(msg))
		binary.BigEndian.PutUint64(buf, uint64(receiverId))
		binary.BigEndian.PutUint64(buf[8:], uint64(0))
		binary.BigEndian.PutUint32(buf[16:], uint32(cmdId))
		binary.BigEndian.PutUint32(buf[20:], uint32(len(msg)))
		copy(buf[16+packet.PacketHeaderSize:], msg)
		_, e := rdsSessPubSub.Do("publish", IpAndPort, buf)

		log.Tracef("redis 订阅发布: 向 topic %s 发送消息", IpAndPort)
		return e
	}
	return err
}

// 发送群聊消息
func SendGroupBuf(topic string, groupId int64, cmdId msg_id.NetMsgId, msg []byte) error {
	buf := make([]byte, 16+packet.PacketHeaderSize+len(msg))
	binary.BigEndian.PutUint64(buf, uint64(0)) //
	binary.BigEndian.PutUint64(buf[8:], uint64(groupId))
	binary.BigEndian.PutUint32(buf[16:], uint32(cmdId))
	binary.BigEndian.PutUint32(buf[20:], uint32(len(msg)))
	copy(buf[16+packet.PacketHeaderSize:], msg)
	_, e := rdsSessPubSub.Do("publish", topic, buf)
	return e
}

// 发送群聊消息
func SendGroupMsg(topic string, groupId int64, cmdId msg_id.NetMsgId, pb proto.Message) error {
	msg, err := proto.Marshal(pb)
	if err == nil {
		buf := make([]byte, 16+packet.PacketHeaderSize+len(msg))
		binary.BigEndian.PutUint64(buf, uint64(0)) //
		binary.BigEndian.PutUint64(buf[8:], uint64(groupId))
		binary.BigEndian.PutUint32(buf[16:], uint32(cmdId))
		binary.BigEndian.PutUint32(buf[20:], uint32(len(msg)))
		copy(buf[16+packet.PacketHeaderSize:], msg)
		_, e := rdsSessPubSub.Do("publish", topic, buf)
		return e
	}

	return err
}
