package main

import (
	"encoding/binary"
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"net/http"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

const (
	maxReadSize = 1024
)

var (
	mapAgentMsgsrv map[string]string
	loginAddr      string
)

type Client struct {
	webSocket *websocket.Conn
	tcpSocket *im_net.Socket
	send      chan []byte
}

func (c *Client) run() {
	defer func() {
		c.webSocket.Close()
	}()

	b := false
	for {
		_, message, err := c.webSocket.ReadMessage()
		if err != nil {
			str := fmt.Sprintf("c.webSocket.ReadMessage error: %v", err)
			log.Errorf(str)
			return
		}

		// 登录服务器只接受客户端一次请求
		if b {
			log.Errorf("发现有个客户端登录时发送 2 次请求, 应该只发送一次;关闭它的连接")
			return
		}

		p := &msg_struct.WebSocketMessage{}
		if !function.ProtoUnmarshal(message, p, "msg_struct.WebSocketMessage") {
			return
		}

		b = true
		s, e := im_net.ConnectSocket("tcp", loginAddr, maxReadSize)
		if e != nil {
			log.Errorf("connect to %s error: %v", loginAddr, e)
			return
		}

		c.tcpSocket = s
		defer c.tcpSocket.Close()
		e1 := c.tcpSocket.SendPbBuf(msg_id.NetMsgId(p.MsgId), p.BinMsg)
		if e1 != nil {
			log.Errorf("send data to login %s error: %v", loginAddr, e1)
		}

		ackBuf, e2 := c.tcpSocket.ReadOne()
		if e2 != nil {
			log.Errorf("read data from login %s error: %v", loginAddr, e2)
			return
		}

		p2 := &msg_struct.L2CGateAddr{}
		if !function.ProtoUnmarshal(ackBuf[8:], p2, "msg_struct.L2CGateAddr") {
			return
		}
		p2.Ip = mapAgentMsgsrv[p2.Ip]

		msg, b2 := function.ProtoMarshal(p2, "msg_struct.L2CGateAddr")
		if b2 == false {
			return
		}

		pid := binary.BigEndian.Uint32(ackBuf)
		c.SendPbBuf(int32(pid), msg)
	}
}

func (c *Client) SendPbBuf(pid int32, msg []byte) error {
	p := &msg_struct.WebSocketMessage{
		MsgId:    pid,
		MsgIdStr: msg_id.NetMsgId_name[pid],
		BinMsg:   msg,
	}

	wsMsgBuf, err2 := proto.Marshal(p)
	if err2 != nil {
		return err2
	}

	return c.webSocket.WriteMessage(websocket.BinaryMessage, wsMsgBuf)
}

func init() {
	// 建立日志文件
	logDir := os.Getenv("IMLOGPATH")
	exits, err := function.PathExists(logDir)
	if err != nil {
		str := fmt.Sprintf("get dir error, %v", err)
		panic(str)
	}

	if !exits {
		os.Mkdir(logDir, os.ModePerm)
	}

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/wslogin", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)

}

func deferFunc() {
	log.Close()
	function.Catch()
}

func main() {
	defer deferFunc()
	wsloginCfg, err1 := config.ReadWSLoginAgentConfig(os.Getenv("IMCONFIGPATH") + "/wslogin.json")
	if nil != err1 {
		log.Errorf("读取配置 wslogin.json 文件出错:%v", err1)
		return
	}
	loginAddr = wsloginCfg.LoginAddr

	mapAgentMsgsrv = map[string]string{}
	for _, v := range wsloginCfg.MsgServerList {
		mapAgentMsgsrv[v.Msgaddr] = v.WSMsgaddr
	}

	http.HandleFunc("/wslogin", wsPage)
	http.ListenAndServe(":12345", nil)
	fmt.Println("vim-go")
}

func wsPage(res http.ResponseWriter, req *http.Request) {
	conn, error := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if error != nil {
		http.NotFound(res, req)
		return
	}

	client := &Client{webSocket: conn, send: make(chan []byte)}

	go client.run()
}
