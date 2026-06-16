package command

import (
	"testing"
)

func TestNewRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry 应返回非 nil")
	}
	if len(r.List()) != 0 {
		t.Fatal("新注册表应为空")
	}
}

func TestRegister_AddsCommand(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{
		Name: "test", Description: "test command",
		Handler: func(string) bool { return true },
	})
	if len(r.List()) != 1 {
		t.Fatalf("应有 1 个命令，实际 %d", len(r.List()))
	}
	if r.List()[0].Name != "test" {
		t.Fatalf("命令名应为 test，实际 %s", r.List()[0].Name)
	}
}

func TestRegister_MultipleCommands(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{Name: "a", Handler: func(string) bool { return true }})
	r.Register(Command{Name: "b", Handler: func(string) bool { return true }})
	if len(r.List()) != 2 {
		t.Fatalf("应有 2 个命令，实际 %d", len(r.List()))
	}
}

func TestHandle_KnownCommand(t *testing.T) {
	called := false
	r := NewRegistry()
	r.Register(Command{
		Name: "hello",
		Handler: func(args string) bool {
			called = true
			if args != "world" {
				t.Fatalf("args 应为 'world'，实际 %q", args)
			}
			return true
		},
	})
	result := r.Handle("/hello world")
	if !result {
		t.Fatal("Handle 应为 true")
	}
	if !called {
		t.Fatal("Handler 未被调用")
	}
}

func TestHandle_CommandWithoutArgs(t *testing.T) {
	called := false
	r := NewRegistry()
	r.Register(Command{
		Name: "ping",
		Handler: func(args string) bool {
			called = true
			if args != "" {
				t.Fatalf("args 应为空，实际 %q", args)
			}
			return true
		},
	})
	result := r.Handle("/ping")
	if !result {
		t.Fatal("Handle 应为 true")
	}
	if !called {
		t.Fatal("Handler 未被调用")
	}
}



func TestHandle_NonCommand(t *testing.T) {
	r := NewRegistry()
	// 不以 / 开头的输入应返回 false
	result := r.Handle("hello world")
	if result {
		t.Fatal("非命令输入应返回 false")
	}
}

func TestHandle_EmptyInput(t *testing.T) {
	r := NewRegistry()
	result := r.Handle("")
	if result {
		t.Fatal("空输入应返回 false")
	}
}

func TestHandle_JustSlash(t *testing.T) {
	r := NewRegistry()
	// 仅 / 会被解析为空命令名→未知命令→返回 true（已处理）
	result := r.Handle("/")
	if !result {
		t.Fatal("仅 / 应返回 true（作为未知命令处理）")
	}
}

func TestList_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{Name: "cmd1", Handler: func(string) bool { return true }})
	list := r.List()
	if len(list) != 1 {
		t.Fatalf("List 长度应为 1，实际 %d", len(list))
	}
	if list[0].Name != "cmd1" {
		t.Fatalf("命令名应为 cmd1，实际 %s", list[0].Name)
	}
}

func TestHandle_MultipleArgs(t *testing.T) {
	var captured string
	r := NewRegistry()
	r.Register(Command{
		Name: "echo",
		Handler: func(args string) bool {
			captured = args
			return true
		},
	})
	r.Handle("/echo foo bar baz")
	if captured != "foo bar baz" {
		t.Fatalf("Args 应为 'foo bar baz'，实际 %q", captured)
	}
}
