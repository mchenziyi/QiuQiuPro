package tool

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCommandStreaming_Success(t *testing.T) {
	shell, arg := detectShell()
	result := runCommandStreaming(5*time.Second, shell, arg, "echo hello_qiuqiu")
	if !strings.Contains(result, "hello_qiuqiu") {
		t.Fatalf("应包含命令输出，实际：%s", result)
	}
	if !strings.Contains(result, "退出码 0") || !strings.Contains(result, "成功") {
		t.Fatalf("应判定成功且退出码 0，实际：%s", result)
	}
}

func TestRunCommandStreaming_Failure(t *testing.T) {
	shell, arg := detectShell()
	result := runCommandStreaming(5*time.Second, shell, arg, "exit 7")
	if !strings.Contains(result, "退出码 7") || !strings.Contains(result, "失败") {
		t.Fatalf("应判定失败且退出码 7，实际：%s", result)
	}
}

func TestRunCommandStreaming_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 下 sleep 语义不同，跳过")
	}
	shell, arg := detectShell()
	start := time.Now()
	result := runCommandStreaming(200*time.Millisecond, shell, arg, "sleep 3")
	if !strings.Contains(result, "超时") {
		t.Fatalf("应判定超时，实际：%s", result)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("超时后应尽快返回，实际耗时 %s", elapsed)
	}
}

func TestRunCommandStreaming_StartError(t *testing.T) {
	result := runCommandStreaming(5*time.Second, "definitely_not_a_real_binary_xyz_123")
	if !strings.Contains(result, "无法执行") {
		t.Fatalf("不存在的命令应提示无法执行，实际：%s", result)
	}
}

func TestFormatShellOutput(t *testing.T) {
	if got := formatShellOutput(""); got != "（无输出）" {
		t.Errorf("空输出应提示（无输出），实际 %q", got)
	}
	if got := formatShellOutput("abc"); got != "abc" {
		t.Errorf("短输出应原样返回，实际 %q", got)
	}
	long := strings.Repeat("x", runShellMaxOutput+100)
	if got := formatShellOutput(long); !strings.Contains(got, "已截断") {
		t.Errorf("超长输出应截断，实际尾部 %q", got[max(0, len(got)-40):])
	}
}

func TestCappedBuffer(t *testing.T) {
	var b bytes.Buffer
	cb := &cappedBuffer{buf: &b, max: 5}

	n, err := cb.Write([]byte("abcdefgh")) // 8 字节，超过 max=5
	if err != nil || n != 8 {
		t.Fatalf("Write 应声明全量写入 (8,nil)，实际 (%d,%v)", n, err)
	}
	if b.String() != "abcde" {
		t.Fatalf("缓冲应被截到 'abcde'，实际 %q", b.String())
	}

	// 已满后继续写：内容全丢弃，但仍声明写入字节数。
	n2, _ := cb.Write([]byte("xyz"))
	if n2 != 3 || b.Len() != 5 {
		t.Fatalf("已满后应丢弃且声明写入 3，实际 n=%d len=%d", n2, b.Len())
	}
}
