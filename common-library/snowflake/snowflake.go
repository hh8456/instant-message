package snowflake

// twitter 雪花算法
// 把时间戳,工作机器ID, 序列号组合成一个 64位 int
// 第一位不用, [2,42]这41位存放时间戳,[43,52]这10位存放机器id,[53,64]最后12位存放序列号

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	machineID     int64 // 机器 id 占10位, 十进制范围是 [ 0, 1023 ]
	sn            int64 // 序列号占 12 位,十进制范围是 [ 0, 4095 ]
	lastTimeStamp int64 // 上次的时间戳(毫秒级), 1秒=1000毫秒, 1毫秒=1000微秒,1微秒=1000纳秒
	lastID        int64
	timeMask      int64
	lock          sync.Mutex
)

func init() {
	lastTimeStamp = time.Now().UnixNano() >> 20 // 除以 1000000, 纳秒转为毫秒
	machineID = 666 << 12                       // 单纯给个吉祥数字
	timeMask = ^(int64(-1) << 41)
}

func SetMachineId(mid int64) {
	// 把机器 id 左移 12 位,让出 12 位空间给序列号使用
	machineID = mid << 12
}

func getSeqFromID(id int64) int64 {
	// [53,64]最后12位存放序列号
	return id & 0xFFF
}

func getTimeFromID(id int64) int64 {
	// [2,42]这41位存放时间戳
	return id >> 22
}

func getCurrentTimestamp() int64 {
	curTimeStamp := time.Now().UnixNano() >> 20 // 除以 1000000, 纳秒转为毫秒
	return curTimeStamp & timeMask              // 获得时间戳的低 41 位
}

// XXX 这个并发生成雪花ID 的代码是根据 https://github.com/beinan/fastid 修改的
// 但是测试发现, 一百个协程并发执行十万次就挂了, 暂时不敢用
func getSnowflakeIdConcurrent() int64 {
	for {
		localLastID := atomic.LoadInt64(&lastID)
		seq := getSeqFromID(localLastID)
		//println("seq =", seq)
		lastIDTime := getTimeFromID(localLastID)
		//println("lastIDTime =", lastIDTime)
		now := getCurrentTimestamp()
		//println("now =", now)
		if now > lastIDTime {
			seq = 0
		} else if seq >= 4095 {
			//time.Sleep(time.Millisecond)
			time.Sleep(time.Duration(0xFFFFF - (time.Now().UnixNano() & 0xFFFFF)))
			//println(seq)
			//println("now =", now)
			//println("lastIDTime =", lastIDTime)
			continue
		} else {
			seq++
		}

		newID := (now << 22) | machineID | seq
		if atomic.CompareAndSwapInt64(&lastID, localLastID, newID) {
			return newID
		}

		time.Sleep(time.Duration(20))
	}
}

// 线程安全版本
// 10个线程并发,各调用 100万次 GetSnowflakeId(), 只花了 10 秒
func GetSnowflakeId() int64 {

	lock.Lock()

	curTimeStamp := time.Now().UnixNano() >> 20 // 除以 1000000, 纳秒转为毫秒

	// 同一毫秒
	if curTimeStamp == lastTimeStamp {
		sn++
		// 序列号占 12 位,十进制范围是 [ 0, 4095 ]
		if sn > 4095 {
			time.Sleep(time.Millisecond)
			curTimeStamp = time.Now().UnixNano() >> 20 // 除以 1000000, 纳秒转为毫秒
			if lastTimeStamp == curTimeStamp {
				time.Sleep(time.Millisecond)
				curTimeStamp = time.Now().UnixNano() >> 20 // 除以 1000000, 纳秒转为毫秒
			}
			lastTimeStamp = curTimeStamp
			sn = 0
		}

		// 取二进制 00111( 41 个 1 ) 和 时间戳进行并操作, 并结果(右数)第 42 位必然是 0
		// 并结果的低 41 位二进制,也是时间戳的低41位
		rightBinValue := curTimeStamp & 0x1FFFFFFFFFF
		// 机器 id 占用10位空间,序列号占用12位空间,所以左移 22 位
		rightBinValue <<= 22

		id := rightBinValue | machineID | sn

		lock.Unlock()
		return id
	}

	if curTimeStamp > lastTimeStamp {
		lastTimeStamp = curTimeStamp
		sn = 0

		// 取二进制 00111( 41 个 1 ) 和 时间戳进行并操作, 结果的第 1 位必然是 0
		// 结果的这 41 位二进制,也是时间戳的低41位
		rightBinValue := curTimeStamp & 0x1FFFFFFFFFF
		// 机器 id 占用10位空间,序列号占用12位空间,所以左移 22 位
		rightBinValue <<= 22

		id := rightBinValue | machineID | sn

		lock.Unlock()
		return id
	}

	if curTimeStamp < lastTimeStamp {
		lock.Unlock()
		return 0
	}

	lock.Unlock()
	return 0
}
