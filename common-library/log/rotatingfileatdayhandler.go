package log

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"strings"
	"time"
)

const (
	kRollPerSeconds = 60 * 60 * 24 // one day
)

var (
	pid  = os.Getpid()
	host = "unknowhost"
)

func init() {
	h, err := os.Hostname()
	if err == nil {
		host = shortHostname(h)
	}
}

func shortHostname(hostname string) string {
	if i := strings.Index(hostname, "."); i >= 0 {
		return hostname[:i]
	}
	return hostname
}

func logFileName(basename string) string {
	now := time.Now()
	name := fmt.Sprintf("%s.%04d%02d%02d-%02d%02d%02d.%s.%d.log",
		basename,
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second(),
		host,
		pid,
	)

	return name
}

type RotatingFileAtDayHandler struct {
	baseName      string
	rollSize      int
	flushInterval int64
	checkEveryN   int
	syncFlush     bool
	count         int
	startofPeriod int64
	lastRoll      int64
	lastFlush     int64
	file          *appendFile
}

// baseName 日志文件的基本名包含全路径 如 "/tmp/test"
// rollSize 每写入rollSize字节日志滚动文件
// flushInterval 刷新文件写入缓存的间隔
// checkEveryN 每写入checkEveryN次 检查文件回滚和缓存刷新
// syncFlush == true flushInterval和checkEveryN 失效
func NewRotatingFileAtDayHandler(baseName string, rollSize int,
	flushInterval int64, checkEveryN int, syncFlush bool) *RotatingFileAtDayHandler {
	hander := &RotatingFileAtDayHandler{
		baseName:      baseName,
		rollSize:      rollSize,
		flushInterval: flushInterval,
		checkEveryN:   checkEveryN,
		syncFlush:     syncFlush,
		count:         0,
		startofPeriod: 0,
		lastRoll:      0,
		lastFlush:     0,
	}
	hander.rollFile()
	return hander
}

func NewDefaultRotatingFileAtDayHandler(baseName string,
	rollSize int) *RotatingFileAtDayHandler {
	//checkEveryN 开发期间默认设置为1 上线后调高已提高处理性能
	return NewRotatingFileAtDayHandler(baseName, rollSize, 3, 1, true)
}

func (self *RotatingFileAtDayHandler) Write(b []byte) (int, error) {
	n, err := self.file.append(b)
	if err != nil {
		return n, err
	}
	if self.file.writtenBytes() > self.rollSize {
		self.rollFile()
	} else {
		self.count++
		if self.count >= self.checkEveryN || self.syncFlush {
			self.count = 0
			now := time.Now().Unix()
			thisPeriod := now / kRollPerSeconds * kRollPerSeconds
			if thisPeriod != self.startofPeriod {
				self.rollFile()
			} else if now-self.lastFlush > self.flushInterval || self.syncFlush {
				self.lastFlush = now
				err = self.file.flush()
			}
		}
	}
	return n, err
}

func (self *RotatingFileAtDayHandler) flush() error {
	return self.file.flush()
}

func (self *RotatingFileAtDayHandler) rollFile() bool {
	fileName := logFileName(self.baseName)
	t := time.Now().Unix()
	start := t / kRollPerSeconds * kRollPerSeconds
	//滚动时间间隔最小为1秒
	if t > self.lastRoll {
		self.lastRoll = t
		self.lastFlush = t
		self.startofPeriod = start
		if self.file != nil {
			self.file.close()
		}
		tmpFile, _ := newAppendFile(fileName)
		self.file = tmpFile
		return true
	}
	return false
}

func (self *RotatingFileAtDayHandler) Close() error {
	return self.file.close()
}

type appendFile struct {
	file          *os.File
	writer        *bufio.Writer
	writtenBytes_ int
}

func newAppendFile(filename string) (*appendFile, error) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		return nil, err
	}
	return &appendFile{
		file:          file,
		writer:        bufio.NewWriterSize(file, 64*1024),
		writtenBytes_: 0,
	}, nil
}

func (self *appendFile) append(b []byte) (int, error) {
	length := len(b)
	remain := length
	offset := 0
	var err error
	for remain > 0 {
		x, err := self.writer.Write(b[offset:])
		if err != nil {
			if err != io.ErrShortWrite {
				break
			}
		}

		offset = offset + x
		remain = length - offset
	}
	self.writtenBytes_ = self.writtenBytes_ + length
	return offset, err
}

func (self *appendFile) writtenBytes() int {
	return self.writtenBytes_
}
func (self *appendFile) flush() error {
	return self.writer.Flush()
}

func (self *appendFile) close() error {
	err := self.writer.Flush()
	for err != nil {
		if err == io.ErrShortWrite {
			err = self.writer.Flush()
		} else {
			break
		}
	}
	if err != nil {
		return err
	}

	return self.file.Close()
}
