package etcd

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

const OnlineUserNumberDir = "OnlineUserNumberDir"

func WatchOnlineChatGroup(cli *clientv3.Client, dir string) {
	dir = strings.TrimRight(dir, "/") + "/"
	curRevision := int64(0)
	kv := clientv3.NewKV(cli)
	for {
		rangeResp, err := kv.Get(context.TODO(), dir, clientv3.WithPrefix()) // 因为要监听目录,所以传送了 clientv3.WithPrefix()
		if err != nil {
			continue

		}

		for _, kv := range rangeResp.Kvs {
			println(string(kv.Key), " :", string(kv.Value))
		}

		curRevision = rangeResp.Header.Revision + 1
		break
	}

	// 监听后续的PUT与DELETE事件
	watcher := clientv3.NewWatcher(cli)
	watchChan := watcher.Watch(context.TODO(), dir, clientv3.WithPrefix(), clientv3.WithRev(curRevision))
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			switch event.Type {
			case mvccpb.PUT:
				// event.Kv.Key 的值是 dir/topic
				// event.Kv.Value 的值是 群号码 集合
				// 表示在某台服务器上有多少个在线群
				//fmt.Println(fmt.Sprintf("PUT 事件: %s - %s", string(event.Kv.Key), string(event.Kv.Value)))

			case mvccpb.DELETE:
			}
		}
	}
}

type OnlineUserNumber struct {
	prefix string
	lock   sync.RWMutex
	m      map[string]int // topic( 服务器 ip ) - 在线人数
}

func NewOnlineUserNumber(prefix string) *OnlineUserNumber {
	prefix = strings.TrimRight(prefix, "/") + "/"
	return &OnlineUserNumber{prefix: prefix, m: map[string]int{}}
}

func (on *OnlineUserNumber) GetMininumLoad() (string, int) {
	on.lock.RLock()
	defer on.lock.RUnlock()

	var r string
	minium := math.MaxInt32
	for topic, numbers := range on.m {
		if numbers < minium {
			r = topic
			minium = numbers
		}
	}
	return strings.TrimLeft(r, on.prefix), minium
}

func (on *OnlineUserNumber) setLoad(topic string, load int) {
	on.lock.Lock()
	on.m[topic] = load
	on.lock.Unlock()
}

func (on *OnlineUserNumber) remove(topic string) {
	on.lock.Lock()
	delete(on.m, topic)
	on.lock.Unlock()
}

func WatchOnlineUserNmuber(cli *clientv3.Client, dir string, on *OnlineUserNumber) {
	dir = strings.TrimRight(dir, "/") + "/"
	curRevision := int64(0)
	kv := clientv3.NewKV(cli)
	for {
		rangeResp, err := kv.Get(context.TODO(), dir, clientv3.WithPrefix()) // 因为要监听目录,所以传送了 clientv3.WithPrefix()
		if err != nil {
			continue
		}

		for _, kv := range rangeResp.Kvs {
			println(string(kv.Key), " :", string(kv.Value))
		}

		curRevision = rangeResp.Header.Revision + 1
		break
	}

	// 监听后续的PUT与DELETE事件
	watcher := clientv3.NewWatcher(cli)
	watchChan := watcher.Watch(context.TODO(), dir, clientv3.WithPrefix(), clientv3.WithRev(curRevision))
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			switch event.Type {
			case mvccpb.PUT:
				// event.Kv.Key 的值是 dir/topic: 即 OnlineUserNumber/192.168.10.240:5001
				// event.Kv.Value 的值是 在线人数
				// 表示在 topic( 192.168.10.240:5001 ) 这台服务器上有多少个在线人数
				//log.Tracef(fmt.Sprintf("etcd put 事件, ip: %s - 在线人数: %s", string(event.Kv.Key), string(event.Kv.Value)))
				num, e := strconv.ParseInt(string(event.Kv.Value), 10, 32)
				if e == nil {
					on.setLoad(string(event.Kv.Key), int(num))
				}

			case mvccpb.DELETE:
				//log.Tracef(fmt.Sprintf("etcd delete 事件, ip: %s - 在线人数: %s", string(event.Kv.Key), string(event.Kv.Value)))
				on.remove(string(event.Kv.Key))
			}
		}
	}
}
