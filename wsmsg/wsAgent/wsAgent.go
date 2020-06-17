package wsAgent

import (
	"encoding/binary"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"io"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

type AgentConn struct {
	webSocket     *websocket.Conn
	tcpSocket     *im_net.Socket
	wsSendChan    chan []byte
	wsCloseSignal chan interface{}
	lock          sync.RWMutex
	isTcpClosed   bool
}

func New(wsConn *websocket.Conn, backEndAddr string) (*AgentConn, error) {
	s, e := im_net.ConnectSocket("tcp", backEndAddr, 1024*1024*10)
	if e != nil {
		log.Errorf("connect to %s error: %v", backEndAddr, e)
		return nil, e
	}

	c := &AgentConn{webSocket: wsConn, tcpSocket: s, wsSendChan: make(chan []byte, 30),
		wsCloseSignal: make(chan interface{})}

	return c, nil
}

func (c *AgentConn) closeTcpSocket() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.isTcpClosed == true {
		return
	}

	c.isTcpClosed = true
	c.tcpSocket.Close()
}

// 接收 webSocket 客户端的消息,转发给后端服务器
func (c *AgentConn) webSocketRecvLoop() {
	defer func() {
		function.Catch()
		c.webSocket.Close()
		c.closeTcpSocket()
		close(c.wsCloseSignal)
	}()

	for {
		_, message, err := c.webSocket.ReadMessage()
		if err != nil {
			str := fmt.Sprintf("c.webSocket.ReadMessage error: %v", err)
			log.Errorf(str)
			return
		}

		p := &msg_struct.WebSocketMessage{}
		if !function.ProtoUnmarshal(message, p, "msg_struct.WebSocketMessage") {
			return
		}

		log.Tracef("收到客户端发来的协议 id: %s", msg_id.NetMsgId_name[p.MsgId])
		c.tcpSocket.SendPbBuf(msg_id.NetMsgId(p.MsgId), p.BinMsg)
	}
}

// 接收后端服务器发来的消息,投递到通道中,让发送协程下发给 webSocket 客户端
func (c *AgentConn) tcpRecvLoop() {
	defer func() {
		function.Catch()
		c.closeTcpSocket()
	}()

	for {
		msg, err := c.tcpSocket.ReadOne()
		if err != nil {
			if err != io.EOF {
				log.Errorf("AgentConn.tcpRecvLoop read error: %s", err)
			}

			return
		}

		select {
		case c.wsSendChan <- msg:

		default:
			log.Errorf("AgentConn.wsSendChan 通道容量满了, 导致客户端接收的消息溢出")
		}
	}
}

// 把数据下发给 webSocket 客户端
func (c *AgentConn) webSocketSendLoop() {
	defer function.Catch()
	for {
		select {
		case msg := <-c.wsSendChan:
			pid := binary.BigEndian.Uint32(msg)
			c.WsSendPbBuf(int32(pid), msg[8:])

		case <-c.wsCloseSignal:
			return

		}
	}
}

func (c *AgentConn) Run() {
	go c.webSocketRecvLoop()
	go c.webSocketSendLoop()
	go c.tcpRecvLoop()
}

func (c *AgentConn) WsSendPbBuf(pid int32, msg []byte) error {
	p := &msg_struct.WebSocketMessage{
		MsgId:  pid,
		BinMsg: msg,
	}

	wsMsgBuf, err2 := proto.Marshal(p)
	if err2 != nil {
		return err2
	}

	return c.webSocket.WriteMessage(websocket.BinaryMessage, wsMsgBuf)
}
