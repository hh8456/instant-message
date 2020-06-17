//debug版本做一些特殊处理

// +build debug

package log

import (
	"os"
)

func (l *Logger) Output(callDepth int, level int, s string) {
	outputBuffer := l.generateOutputBuffer(callDepth+1, level, s)
	l.msg <- outputBuffer
	os.Stdout.Write(outputBuffer)
}
