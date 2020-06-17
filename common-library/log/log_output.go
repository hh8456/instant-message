// +build !debug

package log

func (l *Logger) Output(callDepth int, level int, s string) {
	outputBuffer := l.generateOutputBuffer(callDepth+1, level, s)
	l.msg <- outputBuffer
}
