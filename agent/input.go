package agent

import (
	"bufio"
	"os"
	"strings"
)

// 统一输入：整个程序对 os.Stdin 只用这一个带缓冲的 reader。
//
// 过去主循环用 bufio.Scanner、确认处用 fmt.Scanln，两个读取器同时盯着 os.Stdin。
// bufio 会预读并缓冲，fmt.Scanln 又直接读底层流——粘贴多行 / 管道输入时缓冲会错位，
// 导致确认提示读错、或把后续输入吞掉。收口到单一 reader 即可根除。

// stdin 返回 Agent 统一的输入读取器（未注入时惰性初始化，保证子 Agent / 测试也可用）。
func (a *Agent) stdin() *bufio.Reader {
	if a.in == nil {
		a.in = bufio.NewReader(os.Stdin)
	}
	return a.in
}

// SetInput 注入共享的输入读取器。main 启动时创建一个，读 API Key、主循环、确认共用同一个。
func (a *Agent) SetInput(r *bufio.Reader) { a.in = r }

// ReadLine 从统一输入流读取一行（去掉行尾换行）。ok=false 表示 EOF / 输入结束。
func (a *Agent) ReadLine() (string, bool) {
	line, err := a.stdin().ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && line == "" {
		return "", false
	}
	return line, true
}

// confirm 读取一行 [Y/n] 确认：空行或非 n 视为确认（默认 Yes）；EOF 视为取消（对高危操作更安全）。
func (a *Agent) confirm() bool {
	line, ok := a.ReadLine()
	if !ok {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s != "n" && s != "no"
}
