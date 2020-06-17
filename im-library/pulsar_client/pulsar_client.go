package pulsar_client

import (
	"context"
	"encoding/binary"
	"fmt"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"runtime"

	pulsarLog "github.com/apache/pulsar/pulsar-client-go/logutil"
	"github.com/apache/pulsar/pulsar-client-go/pulsar"
	"github.com/gogo/protobuf/proto"
)

type producerWrapper struct {
	producer pulsar.Producer
	ctx      context.Context
}

type consumerWrapper struct {
	consumer pulsar.Consumer
	ctx      context.Context
}

var (
	pulsarClient          pulsar.Client
	mapProducter          map[string]*producerWrapper
	mapConsumer           map[string]*consumerWrapper
	mapPlusarMsgChannel   map[string]chan pulsar.Message
	sliceSubscribeTopices []string
)

func loggerFunc(level pulsarLog.LoggerLevel, file string, line int, message string) {
	str := fmt.Sprintf("custom logger func, file: %s, line: %d, message: %s", file, line, message)
	switch level {
	case pulsarLog.DEBUG:
		log.Debug(str)
	case pulsarLog.INFO:
		log.Info(str)
	case pulsarLog.WARN:
		log.Warn(str)
	case pulsarLog.ERROR:
		log.Error(str)
	}
}

func init() {
	mapProducter = map[string]*producerWrapper{}
	mapConsumer = map[string]*consumerWrapper{}
	mapPlusarMsgChannel = map[string]chan pulsar.Message{}
	sliceSubscribeTopices = []string{}
}

/*
struct pulsar.ClientOptions 中各参数意义

**Args**

* `service_url`: The Pulsar service url eg: pulsar://my-broker.com:6650/
**Options**
* `authentication`:
Set the authentication provider to be used with the broker. For example:
`AuthenticationTls` or `AuthenticationAthenz`

* `operation_timeout_seconds`:
Set timeout on client operations (subscribe, create producer, close,
unsubscribe).

* `io_threads`:
Set the number of IO threads to be used by the Pulsar client.

* `message_listener_threads`:
Set the number of threads to be used by the Pulsar client when
delivering messages through message listener. The default is 1 thread
per Pulsar client. If using more than 1 thread, messages for distinct
`message_listener`s will be delivered in different threads, however a
single `MessageListener` will always be assigned to the same thread.

* `concurrent_lookup_requests`:
Number of concurrent lookup-requests allowed on each broker connection
to prevent overload on the broker.

* `log_conf_file_path`:
Initialize log4cxx from a configuration file.

* `use_tls`:
Configure whether to use TLS encryption on the connection. This setting
is deprecated. TLS will be automatically enabled if the `serviceUrl` is
set to `pulsar+ssl://` or `https://`

* `tls_trust_certs_file_path`:
Set the path to the trusted TLS certificate file.

* `tls_allow_insecure_connection`:
Configure whether the Pulsar client accepts untrusted TLS certificates
from the broker.
"""
*/

func Init(url string, publishTopices []string, subscribeTopices []string) {
	if pulsarClient != nil {
		return
	}

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:                    url,
		MessageListenerThreads: runtime.NumCPU(),
		Logger:                 loggerFunc,
	})

	if err != nil {
		str := fmt.Sprintf("初始化 pulsar client 对象失败: %v", err)
		panic(str)
	}

	pulsarClient = client

	publish(publishTopices)
	subscribe(subscribeTopices)
	for _, v := range subscribeTopices {
		sliceSubscribeTopices = append(sliceSubscribeTopices, v)
	}
}

func Close() {
	for k, v := range mapProducter {
		v.producer.Close()
		delete(mapProducter, k)
	}

	for k, v := range mapConsumer {
		v.consumer.Close()
		delete(mapConsumer, k)
	}

	for k, v := range mapPlusarMsgChannel {
		close(v)
		delete(mapPlusarMsgChannel, k)
	}
}

// 发布若干个主题,每个主题有一个 producer
func publish(topices []string) {
	for _, v := range topices {
		p := newProducter(v)
		pw := &producerWrapper{producer: p, ctx: context.Background()}
		mapProducter[v] = pw
	}
}

// 订阅若干个主题, 每个主题有一个 consumer
func subscribe(topices []string) {
	for _, v := range topices {
		c := newConsumer(v)
		cw := &consumerWrapper{consumer: c, ctx: context.Background()}
		mapConsumer[v] = cw

		channel := make(chan pulsar.Message, 10000)
		mapPlusarMsgChannel[v] = channel

		go func() {
			for {
				msg, err := c.Receive(cw.ctx)
				if err == nil {
					if msg != nil {
						channel <- msg
					} else {
						log.Errorf("receive nil msg from topic %s, 接收协程退出,不再订阅该主题", cw.consumer.Topic())
						return
					}
				} else {
					log.Errorf("receive topic %s error: %v, 接收协程退出,不再订阅该主题", cw.consumer.Topic(), err)
					return
				}

				c.AckID(msg.ID())
			}
		}()
	}
}

func newProducter(topic string) pulsar.Producer {
	po := pulsar.ProducerOptions{
		Topic: topic,
	}

	producer, err := pulsarClient.CreateProducer(po)
	if err != nil {
		str := fmt.Sprintf("生成 pulsar producter 对象失败: %v", err)
		panic(str)
	}

	return producer
}

func newConsumer(topic string) pulsar.Consumer {
	co := pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: topic,
		Type:             pulsar.Exclusive,
	}

	consumer, err := pulsarClient.Subscribe(co)
	if err != nil {
		str := fmt.Sprintf("生成 pulsar consumer 对象失败: %v", err)
		panic(str)
	}

	return consumer
}

func Send(topic string, msg []byte) error {
	v, ok := mapProducter[topic]
	if ok {
		return v.producer.Send(v.ctx, pulsar.ProducerMessage{
			Payload: msg,
		})
	}

	return nil
}

func SendPbMsg(topic string, receiverId int64, cmdId msg_id.NetMsgId, pb proto.Message) error {
	msg, err := proto.Marshal(pb)
	if err == nil {
		buf := make([]byte, 16+packet.PacketHeaderSize+len(msg))
		binary.BigEndian.PutUint64(buf, uint64(receiverId))
		binary.BigEndian.PutUint64(buf[8:], uint64(0))
		binary.BigEndian.PutUint32(buf[16:], uint32(cmdId))
		binary.BigEndian.PutUint32(buf[20:], uint32(len(msg)))
		copy(buf[16+packet.PacketHeaderSize:], msg)
		return Send(topic, buf)
	} else {
		return err
	}
}

// 发送群聊消息
func SendGroupBuf(topic string, groupId int64, cmdId msg_id.NetMsgId, msg []byte) error {
	buf := make([]byte, 16+packet.PacketHeaderSize+len(msg))
	binary.BigEndian.PutUint64(buf, uint64(0)) //
	binary.BigEndian.PutUint64(buf[8:], uint64(groupId))
	binary.BigEndian.PutUint32(buf[16:], uint32(cmdId))
	binary.BigEndian.PutUint32(buf[20:], uint32(len(msg)))
	copy(buf[16+packet.PacketHeaderSize:], msg)
	return Send(topic, buf)
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
		return Send(topic, buf)
	} else {
		return err
	}
}

// 获得订阅的主题
func GetSubscribeTopices() []string {
	s := []string{}
	for _, v := range sliceSubscribeTopices {
		s = append(s, v)
	}

	return s
}

func GetMsgFromTopic(topic string) <-chan pulsar.Message {
	v, ok := mapPlusarMsgChannel[topic]
	if ok {
		return v
	}

	return nil
}
