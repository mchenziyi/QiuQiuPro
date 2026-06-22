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

// ReadLine 从统一输入流读取一行（主循环用）。
// Ctrl+C 时不会退出进程，而是提示后重新等待输入。
func (a *Agent) ReadLine() (string, bool) {
	return a.readLine(false)
}

func (a *Agent) readLine(cancelOnInterrupt bool) (string, bool) {
	for {
		line, err := a.stdin().ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if err != nil {
			if atomic.LoadInt32(&a.interrupted) != 0 {
				atomic.StoreInt32(&a.interrupted, 0)
				if cancelOnInterrupt {
					return "", false
				}
				a.noticef("\n  ⚡ 已中断\n")
				continue
			}
			if line == "" {
				return "", false
			}
		}
		return line, true
	}
}

// SetConfirmChan 设置 Web 模式的确认通道。设置后 GateConfirm 将通过通道等待用户批准，
// 而非读取 stdin。传入 nil 恢复 stdin 模式。
func (a *Agent) SetConfirmChan(ch chan bool) { a.confirmChan = ch }

func (a *Agent) confirm() bool {
	if a.confirmChan != nil {
		return <-a.confirmChan
	}
	line, ok := a.readLine(true)
	if !ok {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s != "n" && s != "no"
}

func (a *Agent) Confirm() bool { return a.confirm() }
