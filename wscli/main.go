package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"net/url"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "192.168.10.240:12345", "http service address")

func main() {
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	go timeWriter(conn)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			return
		}

		fmt.Printf("received: %s\n", message)
	}

	fmt.Println("vim-go")
}

func timeWriter(conn *websocket.Conn) {
	msg := &msg_struct.C2LGateAddr{
		AccountId: 123456,
	}

	buf, _ := proto.Marshal(msg)
	//any := ptypes.MarshalAny(msg)

	r := make([]byte, 4+len(buf))
	binary.BigEndian.PutUint32(r, uint32(msg_id.NetMsgId_c2l_gate_addr))
	copy(r[4:], buf)

	for {
		time.Sleep(time.Second * 2)
		conn.WriteMessage(websocket.BinaryMessage, buf)
	}
}
