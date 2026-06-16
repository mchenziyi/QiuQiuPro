package agent

import (
	"bufio"
	"os"
	"strings"
	"sync/atomic"
)

// 统一输入：整个程序对 os.Stdin 只用这一个带缓冲的 reader。

func (a *Agent) stdin() *bufio.Reader {
	if a.in == nil {
		a.in = bufio.NewReader(os.Stdin)
	}
	return a.in
}

func (a *Agent) SetInput(r *bufio.Reader) { a.in = r }

// ReadLine 从统一输入流读取一行。按下 Ctrl+C 时 ReadString 返回错误，
// 检查 interrupted 标记——若为 1 则重置并继续等待输入，不会被当成 EOF。
func (a *Agent) ReadLine() (string, bool) {
	for {
		line, err := a.stdin().ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if err != nil {
			if atomic.LoadInt32(&a.interrupted) == 1 {
				atomic.StoreInt32(&a.interrupted, 0)
				continue
			}
			if line == "" {
				return "", false
			}
		}
		return line, true
	}
}

func (a *Agent) confirm() bool {
	line, ok := a.ReadLine()
	if !ok {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s != "n" && s != "no"
}

func (a *Agent) Confirm() bool { return a.confirm() }
