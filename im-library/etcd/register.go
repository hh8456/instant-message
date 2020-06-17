package etcd

import (
	"context"
	"fmt"
	"instant-message/common-library/log"
	"instant-message/msg-server/clientManager"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
)

func RegisterOnlineChatGroup(cli *clientv3.Client, dir, localTopic string) {
	dir = strings.TrimRight(dir, "/") + "/"
	key := dir + localTopic

	var lastValue string
	kv := clientv3.NewKV(cli)
	lease := clientv3.NewLease(cli)
	var curLeaseId clientv3.LeaseID = 0
	i := int64(0)
	for {
		if curLeaseId == 0 {
			leaseResp, err := lease.Grant(context.TODO(), 60)
			if err != nil {
				goto SLEEP
			}

			value := "xxx"

			if value == lastValue {
				goto SLEEP
			}

			lastValue = value

			if _, err := kv.Put(context.TODO(), key, value, clientv3.WithLease(leaseResp.ID)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				goto SLEEP
			}
			curLeaseId = leaseResp.ID
		} else {
			if i%15 == 0 {
				if _, err := lease.KeepAliveOnce(context.TODO(), curLeaseId); err == rpctypes.ErrLeaseNotFound {
					curLeaseId = 0
					continue
				}
			}

			value := "xxx"
			if value == lastValue {
				goto SLEEP
			}

			lastValue = value
			if _, err := kv.Put(context.TODO(), key, value, clientv3.WithLease(curLeaseId)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				goto SLEEP
			}
		}
	SLEEP:
		i++
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func RegisterOnlineUserNumber(cli *clientv3.Client, dir, localTopic string) {
	dir = strings.TrimRight(dir, "/") + "/"
	key := dir + localTopic

	var lastValue int
	kv := clientv3.NewKV(cli)
	lease := clientv3.NewLease(cli)
	var curLeaseId clientv3.LeaseID = 0
	i := int64(0)
	for {
		if curLeaseId == 0 {
			leaseResp, err := lease.Grant(context.TODO(), 60)
			if err != nil {
				goto SLEEP
			}

			value := clientManager.GetOnlineUserNumber()

			if value == lastValue {
				goto SLEEP
			}

			lastValue = value

			if _, err := kv.Put(context.TODO(), key, strconv.Itoa(value), clientv3.WithLease(leaseResp.ID)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				fmt.Println("etcd put error, %s", err.Error())
				goto SLEEP
			}
			curLeaseId = leaseResp.ID
		} else {
			if i%15 == 0 {
				if _, err := lease.KeepAliveOnce(context.TODO(), curLeaseId); err == rpctypes.ErrLeaseNotFound {
					curLeaseId = 0
					continue
				}
			}

			value := clientManager.GetOnlineUserNumber()
			if value == lastValue {
				goto SLEEP
			}

			lastValue = value
			if _, err := kv.Put(context.TODO(), key, strconv.Itoa(value), clientv3.WithLease(curLeaseId)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				fmt.Println("etcd put error, %s", err.Error())
				goto SLEEP
			}
		}
	SLEEP:
		i++
		time.Sleep(time.Duration(1) * time.Second)
	}
}

// 这个函数用来测试的
func RegisterOnlineUserNumberTest(cli *clientv3.Client, dir, localTopic string, value int) {
	dir = strings.TrimRight(dir, "/") + "/"
	key := dir + localTopic

	var lastValue int
	kv := clientv3.NewKV(cli)
	lease := clientv3.NewLease(cli)
	var curLeaseId clientv3.LeaseID = 0
	i := int64(0)
	for {
		if curLeaseId == 0 {
			leaseResp, err := lease.Grant(context.TODO(), 60)
			if err != nil {
				goto SLEEP
			}

			if value == lastValue {
				goto SLEEP
			}

			lastValue = value

			if _, err := kv.Put(context.TODO(), key, strconv.Itoa(value), clientv3.WithLease(leaseResp.ID)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				fmt.Println("etcd put error, %s", err.Error())
				goto SLEEP
			}
			curLeaseId = leaseResp.ID
		} else {
			if i%15 == 0 {
				if _, err := lease.KeepAliveOnce(context.TODO(), curLeaseId); err == rpctypes.ErrLeaseNotFound {
					curLeaseId = 0
					continue
				}
				value++
			}

			if value == lastValue {
				goto SLEEP
			}

			lastValue = value

			if _, err := kv.Put(context.TODO(), key, strconv.Itoa(value), clientv3.WithLease(curLeaseId)); err != nil {
				log.Errorf("etcd put error, %s", err.Error())
				fmt.Println("etcd put error, %s", err.Error())
				goto SLEEP
			}
		}
	SLEEP:
		i++
		time.Sleep(time.Duration(1) * time.Second)
	}
}
