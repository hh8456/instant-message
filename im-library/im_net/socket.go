package im_net

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"io"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	defTimeoutDuration = time.Minute
)

var ErrOverMaxReadingSize = errors.New("over max reading size")

type Socket struct {
	conn                 net.Conn
	recvBuf              []byte
	reader               *bufio.Reader
	maxReadSize          uint32
	timeoutReadDuration  time.Duration
	timeoutWriteDuration time.Duration
}

func CreateSocket(conn net.Conn, maxReadSize uint32) *Socket {
	return &Socket{
		conn:                 conn,
		recvBuf:              make([]byte, packet.PacketHeaderSize),
		reader:               bufio.NewReader(conn),
		maxReadSize:          maxReadSize,
		timeoutReadDuration:  defTimeoutDuration,
		timeoutWriteDuration: defTimeoutDuration,
	}
}

func (s *Socket) ReadWithCallback(callback func([]byte)) error {
	for {
		b, e := s.read()
		if e != nil {
			return e
		}
		callback(b)
	}
}

func (s *Socket) ReadOne() ([]byte, error) {
	b, e := s.read()
	if e != nil {
		return nil, e
	}
	return b, nil
}

func (s *Socket) Write(msg []byte) error {
	_, e := s.conn.Write(msg)
	return e
}

func (s *Socket) read() ([]byte, error) {
	s.conn.SetReadDeadline(time.Now().Add(s.timeoutReadDuration))
	if _, err := io.ReadFull(s.reader, s.recvBuf[:packet.PacketHeaderSize]); err != nil {
		return nil, err
	}

	msgLen := binary.BigEndian.Uint32(s.recvBuf[4:])

	if msgLen > s.maxReadSize {
		return nil, fmt.Errorf("over max len:%v/%v, msgid:%v",
			binary.BigEndian.Uint32(s.recvBuf[:4]), msgLen, s.maxReadSize)
		//return nil, ErrOverMaxReadingSize
	}

	s.conn.SetReadDeadline(time.Now().Add(s.timeoutReadDuration))

	length := packet.PacketHeaderSize + int(msgLen)
	buf := make([]byte, length)

	copy(buf, s.recvBuf)
	if _, err := io.ReadFull(s.reader, buf[packet.PacketHeaderSize:length]); err != nil {
		return nil, err
	}

	return buf, nil
}

func (s *Socket) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Socket) Close() {
	s.conn.Close()
}

func (s *Socket) SendBuf(msg []byte) error {
	return s.Write(msg)
}

func (s *Socket) SendPbMsg(pid msg_id.NetMsgId, pbMsg proto.Message) error {
	msg, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}

	msgLen := len(msg)
	localBuf := make([]byte, packet.PacketHeaderSize+msgLen)
	binary.BigEndian.PutUint32(localBuf, uint32(pid))
	binary.BigEndian.PutUint32(localBuf[4:], uint32(msgLen))
	copy(localBuf[packet.PacketHeaderSize:], msg)
	return s.Write(localBuf)
}

func (s *Socket) SendPbBuf(pid msg_id.NetMsgId, msg []byte) error {
	msgLen := len(msg)
	localBuf := make([]byte, packet.PacketHeaderSize+msgLen)
	binary.BigEndian.PutUint32(localBuf, uint32(pid))
	binary.BigEndian.PutUint32(localBuf[4:], uint32(msgLen))
	copy(localBuf[packet.PacketHeaderSize:], msg)
	return s.Write(localBuf)
}
