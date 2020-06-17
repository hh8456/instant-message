package packet

import "encoding/binary"

// 逻辑包的包头
type PacketHeader struct {
	id     int32 // 消息 id
	length int32 // data 的长度
}

const (
	// PacketHeader 的大小
	PacketHeaderSize  int = 8
	PacketMaxSize_16K int = 16384                                // 逻辑包最大长度, 包含包头
	PacketMaxBodySize int = PacketMaxSize_16K - PacketHeaderSize // 逻辑体的最大长度,去除包头
	BufSize_128byte   int = 128
	BufSize_256byte   int = 256
	BufSize_512byte   int = 512
	BufSize_1024byte  int = 1024
	BufSize_2K        int = 2048
	BufSize_4K        int = 4096
	BufSize_8K        int = 8192
	BufSize_10K       int = 10240
	BufSize_1MB       int = 1024 * 1024     // 内存池中的块大小最大是 1MB
	BufSize_4MB       int = 4 * 1024 * 1024 // 服务器之间一次通信最多传送 4MB 的数据
	// SliceLength_30    int = 30              // gate server 上,可以向每个客户端连续发送接收30个逻辑包
)

// 逻辑包结构体
type logicPacket struct {
	header PacketHeader
	data   []byte // 二进制消息
}

// 驱动帧的帧头
type FrameHeader struct {
	Serial_Number int // 唯一序列号, 从 1 开始计算, 第1帧的唯一序列号就是 1, 第n帧是的唯一序列号是 n

	Connid_1 uint32
	Connid_2 uint32
	Data     []byte // 二进制消息,原封不动下发给双方
}

// PackByteMsg 打包已转换为byte的proto msg
func PackByteMsg(msgID uint32, msgContent []byte) []byte {
	msgContentLen := len(msgContent)

	buf := make([]byte, PacketHeaderSize+msgContentLen)

	binary.BigEndian.PutUint32(buf, msgID)
	binary.BigEndian.PutUint32(buf[4:], uint32(msgContentLen))
	copy(buf[8:], msgContent)

	return buf
}
