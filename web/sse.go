// Package web 提供 QiuQiuPro 的 Web UI 后端：SSE 事件流 + HTTP API。
//
// 架构：
//
//	SSESink 实现 agent.Sink，Agent 运行时的所有事件经它广播到所有 SSE 客户端。
//	HTTP Server 提供 /api/events（SSE）、/api/send、/api/interrupt、/api/state 等端点。
//	前端为单 HTML 文件，内嵌 CSS+JS，通过 go:embed 打包到二进制中。
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"agentdemo/agent"
)

// ----- SSE 事件类型（对应 ui-spec.md 的事件合约）-----

type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (e SSEEvent) Marshal() []byte {
	data, _ := json.Marshal(e.Data)
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, string(data)))
}

// ----- SSESink -----

// SSESink 实现 agent.Sink，将 Agent 运行事件广播到所有 SSE 客户端。
// 并发安全。
type SSESink struct {
	mu       sync.RWMutex
	clients  map[chan []byte]struct{}
	closed   bool
	onClose  func()
}

// NewSSESink 创建一个 SSESink。
func NewSSESink() *SSESink {
	return &SSESink{
		clients: make(map[chan []byte]struct{}),
	}
}

// OnClose 注册清理回调（例如 HTTP Server 退出时关闭所有 SSE 连接）。
func (s *SSESink) OnClose(fn func()) { s.onClose = fn }

// Subscribe 创建一个新的 SSE 客户端通道。调用方负责在连接关闭时调用 Unsubscribe。
func (s *SSESink) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unsubscribe 移除 SSE 客户端通道并关闭它。
func (s *SSESink) Unsubscribe(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

// Emit 实现 agent.Sink 接口，将事件广播到所有 SSE 客户端。
func (s *SSESink) Emit(ev agent.Event) {
	msgs := s.eventToSSE(ev)
	if len(msgs) == 0 {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return
	}
	for ch := range s.clients {
		for _, msg := range msgs {
			select {
			case ch <- msg:
			default:
				// 客户端消费太慢，丢弃防止阻塞 Agent
			}
		}
	}
}

// eventToSSE 将 agent.Event 转换为 0-N 条 SSE 消息。
func (s *SSESink) eventToSSE(ev agent.Event) [][]byte {
	switch ev.Kind {
	case agent.EventToken:
		return [][]byte{SSEEvent{Type: "assistant_delta", Data: map[string]string{"text": ev.Text}}.Marshal()}

	case agent.EventReasoning:
		return [][]byte{SSEEvent{Type: "reasoning_delta", Data: map[string]string{"text": ev.Text}}.Marshal()}

	case agent.EventToolCall:
		var argsObj interface{}
		if err := json.Unmarshal([]byte(ev.Text), &argsObj); err != nil {
			argsObj = ev.Text
		}
		return [][]byte{SSEEvent{
			Type: "tool_call",
			Data: map[string]interface{}{
				"id":   ev.ID,
				"name": ev.Name,
				"arguments": argsObj,
			},
		}.Marshal()}

	case agent.EventToolResult:
		data := map[string]interface{}{
			"id":     ev.ID,
			"name":   ev.Name,
			"result": ev.Text,
		}
		if ev.Extra != nil {
			if d, ok := ev.Extra["diff"]; ok {
				data["diff"] = d
			}
		}
		return [][]byte{SSEEvent{Type: "tool_result", Data: data}.Marshal()}

	case agent.EventNotice:
		// 跳过 verbose 通知（如缓存诊断日志），只在控制台展示
		if ev.Verbose {
			return nil
		}
		return [][]byte{SSEEvent{Type: "notice", Data: map[string]string{"text": ev.Text}}.Marshal()}

	case agent.EventConfirmRequest:
		return [][]byte{SSEEvent{
			Type: "confirm_request",
			Data: map[string]interface{}{
				"tool_name": ev.Name,
				"arguments": ev.Text,
			},
		}.Marshal()}

	default:
		return nil
	}
}

// Broadcast 向所有 SSE 客户端发送一条原始消息（已编码的 SSE 帧）。
// 调用方负责消息格式正确。线程安全。
func (s *SSESink) Broadcast(msg []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return
	}
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// broadcastState 推送当前 Agent 状态快照给所有 SSE 客户端。
func (s *SSESink) broadcastState(st StateSnapshot) {
	data := SSEEvent{Type: "state", Data: st}.Marshal()
	s.Broadcast(data)
}

// Close 关闭所有 SSE 连接。
func (s *SSESink) Close() {
	s.mu.Lock()
	s.closed = true
	for ch := range s.clients {
		close(ch)
	}
	s.clients = nil
	s.mu.Unlock()
	if s.onClose != nil {
		s.onClose()
	}
}

// ----- StateSnapshot -----

// StateSnapshot 是 /api/state 和 SSE state 事件的数据结构。
type StateSnapshot struct {
	Mode      string `json:"mode"`
	Skill     string `json:"skill"`
	SessionID string `json:"session_id"`
	PlanMode  bool   `json:"plan_mode"`
	Running   bool   `json:"running"`
	CacheHit  int64  `json:"cache_hit"`
	CacheMiss int64  `json:"cache_miss"`
	TokensIn  int64  `json:"tokens_in"`
	TokensOut int64  `json:"tokens_out"`
}

// ----- Server -----

// Server 是 QiuQiuPro Web UI 的 HTTP 服务。
type Server struct {
	agent      *agent.Agent
	sink       *SSESink
	mux        *http.ServeMux
	srv        *http.Server
	mu         sync.Mutex
	running    atomic.Bool
	confirmCh  chan bool
	runMu      sync.Mutex // 确保同一时间只有一个 Run 在执行
}

// NewServer 创建一个 Web UI Server。
func NewServer(a *agent.Agent) *Server {
	confirmCh := make(chan bool, 1)
	s := &Server{
		agent:     a,
		sink:      NewSSESink(),
		mux:       http.NewServeMux(),
		confirmCh: confirmCh,
	}
	a.SetSink(s.sink)
	// Web 模式使用通道进行高危确认，替代 CLI 的 stdin 确认。
	a.SetConfirmChan(confirmCh)
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/send", s.handleSend)
	s.mux.HandleFunc("/api/interrupt", s.handleInterrupt)
	s.mux.HandleFunc("/api/confirm", s.handleConfirm)
	s.mux.HandleFunc("/api/state", s.handleState)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/sessions/switch", s.handleSessionSwitch)
	s.mux.HandleFunc("/api/sessions/new", s.handleSessionNew)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/tools", s.handleTools)
	s.mux.HandleFunc("/", s.handleStatic)
}

// ListenAndServe 监听 addr 并启动 HTTP 服务。
func (s *Server) ListenAndServe(addr string) error {
	s.mu.Lock()
	s.srv = &http.Server{Addr: addr, Handler: s.mux}
	s.mu.Unlock()
	display := addr
	if display[0] == ':' {
		display = "localhost" + display
	}
	log.Printf("🌐 QiuQiuPro Web UI: http://%s\n", display)
	return s.srv.ListenAndServe()
}

// Shutdown 优雅关闭 HTTP 服务和 SSE 连接。
func (s *Server) Shutdown(ctx context.Context) error {
	s.sink.Close()
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv != nil {
		return srv.Shutdown(ctx)
	}
	return nil
}

// /api/events — SSE 端点
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.sink.Subscribe()
	defer s.sink.Unsubscribe(ch)

	// 连接后立即推送当前状态
	s.sink.broadcastState(s.buildState())

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "%s", msg)
			flusher.Flush()
		}
	}
}

// POST /api/send — 发送用户输入
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}

	// 拦截模式切换命令，直接设置 mode 不经过 LLM
	if strings.HasPrefix(req.Text, "/mode ") {
		mode := strings.TrimSpace(strings.TrimPrefix(req.Text, "/mode "))
		if mode == "plan" || mode == "ask" {
			s.runMu.Lock()
			s.agent.SetMode(mode)
			s.runMu.Unlock()
			s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "已切换到 " + mode + " 模式"}}.Marshal())
			s.sink.broadcastState(s.buildState())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "mode": mode})
			return
		}
	}

	// 在后台 goroutine 中执行
	go func() {
		s.sink.Broadcast(SSEEvent{Type: "user_message", Data: map[string]string{"text": req.Text}}.Marshal())

		s.running.Store(true)
		s.sink.broadcastState(s.buildState())

		s.runMu.Lock()
		defer s.runMu.Unlock()
		defer func() {
			s.running.Store(false)
			s.sink.Broadcast(SSEEvent{Type: "done", Data: struct{}{}}.Marshal())
			s.sink.broadcastState(s.buildState())
		}()

		if s.agent.CurrentMode() == "plan" {
			s.runPlanFlow(req.Text)
		} else {
			_, err := s.agent.Run(context.Background(), req.Text)
			if err != nil {
				s.sink.Broadcast(SSEEvent{Type: "error", Data: map[string]string{"text": err.Error()}}.Marshal())
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// runPlanFlow 执行 Plan 模式的全流程：调研 → 生成方案 → 审批 → 执行
func (s *Server) runPlanFlow(goal string) {
	ctx := context.Background()

	s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "📋 正在调研方案...（只读模式，不会修改代码）"}}.Marshal())
	s.agent.SetPlanMode(true)

	research, err := s.agent.Run(ctx, goal)
	if err != nil {
		s.sink.Broadcast(SSEEvent{Type: "error", Data: map[string]string{"text": "调研失败: " + err.Error()}}.Marshal())
		s.agent.SetPlanMode(false)
		return
	}

	planGoal := goal
	if strings.TrimSpace(research) != "" {
		planGoal = goal + "\n\n调研摘要（供规划参考）：\n" + research
	}

	s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "📋 正在生成执行计划..."}}.Marshal())
	plan, err := s.agent.GeneratePlan(ctx, planGoal)
	if err != nil {
		s.sink.Broadcast(SSEEvent{Type: "error", Data: map[string]string{"text": "规划失败: " + err.Error()}}.Marshal())
		s.agent.SetPlanMode(false)
		return
	}
	plan, _ = s.agent.ReviewPlan(ctx, plan)

	// 展示方案，等待用户审批
	s.sink.Broadcast(SSEEvent{Type: "plan_proposal", Data: map[string]interface{}{
		"research": research,
		"plan":     plan,
	}}.Marshal())

	// 发送确认请求
	s.sink.Broadcast(SSEEvent{Type: "confirm_request", Data: map[string]interface{}{
		"tool_name": "plan_execution",
		"arguments": "批准执行此方案",
		"reason":    "方案确定后将开始执行各步骤",
	}}.Marshal())

	// 等待审批
	approved := <-s.confirmCh
	if !approved {
		s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "❌ 方案已取消"}}.Marshal())
		s.agent.SetPlanMode(false)
		return
	}

	s.agent.SetPlanMode(false)
	s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "✅ 方案已批准，开始执行..."}}.Marshal())

	if err := s.agent.ExecutePlan(ctx, plan); err != nil {
		s.sink.Broadcast(SSEEvent{Type: "error", Data: map[string]string{"text": "执行失败: " + err.Error()}}.Marshal())
		return
	}
	s.sink.Broadcast(SSEEvent{Type: "notice", Data: map[string]string{"text": "🎉 执行完成！"}}.Marshal())
}

// POST /api/interrupt — 中断当前执行
func (s *Server) handleInterrupt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	s.agent.Interrupt()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/confirm — 确认或拒绝高危工具
func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	// 将确认结果写入通道，Agent 阻塞的 confirm() 收到后继续执行
	select {
	case s.confirmCh <- req.Approve:
	default:
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GET /api/state — 查询当前状态快照
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.buildState())
}

func (s *Server) buildState() StateSnapshot {
	hit, miss := s.agent.SessionCacheStats()
	return StateSnapshot{
		Mode:      s.agent.CurrentMode(),
		Skill:     s.agent.CurrentSkillName(),
		SessionID: s.agent.SessionID(),
		PlanMode:  s.agent.IsPlanMode(),
		Running:   s.running.Load(),
		CacheHit:  hit,
		CacheMiss: miss,
	}
}

// SessionInfo 是 /api/sessions 返回的会话摘要。
type SessionInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Time    int64  `json:"time"`
	Running bool   `json:"running"`
}

// GET /api/sessions — 列出历史会话
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessionsDir, _ := os.UserHomeDir()
	sessionsDir += "/.qiuqiu/sessions"
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SessionInfo{})
		return
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SessionInfo{})
		return
	}

	var sessions []SessionInfo = make([]SessionInfo, 0)
	currentID := s.agent.SessionID()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".ckpt") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".ckpt")
		title := extractSessionTitle(filepath.Join(sessionsDir, e.Name()))
		info, _ := e.Info()
		sessions = append(sessions, SessionInfo{
			ID:      id,
			Title:   title,
			Time:    info.ModTime().Unix(),
			Running: id == currentID,
		})
	}
	// 按时间降序
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].Time > sessions[i].Time {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// POST /api/sessions/switch — 切换会话
func (s *Server) handleSessionSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if ok := s.agent.SwitchSession(req.SessionID); !ok {
		http.Error(w, "会话不存在", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "session_id": req.SessionID})
}

// POST /api/sessions/new — 新建会话
func (s *Server) handleSessionNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.agent.ResetSession()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "session_id": s.agent.SessionID()})
}

// GET /api/history — 获取当前会话消息列表
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	msgs := s.agent.SessionMessages()
	type ToolCallInfo struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	type Msg struct {
		Role             string         `json:"role"`
		Content          string         `json:"content"`
		ReasoningContent string         `json:"reasoning_content,omitempty"`
		Tool             string         `json:"tool,omitempty"`
		ToolCallID       string         `json:"tool_call_id,omitempty"`
		ToolCalls        []ToolCallInfo `json:"tool_calls,omitempty"`
	}
	var out []Msg = make([]Msg, 0)
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" || m.Role == "tool" {
			msg := Msg{
				Role:             m.Role,
				Content:          m.Content,
				ReasoningContent: m.ReasoningContent,
				Tool:             m.Name,
				ToolCallID:       m.ToolCallID,
			}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, ToolCallInfo{
					ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
				})
			}
			out = append(out, msg)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// GET /api/tools — 返回当前可用工具列表
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	tools := s.agent.ListTools()
	type ToolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ReadOnly    bool   `json:"read_only"`
	}
	var out []ToolInfo
	for _, t := range tools {
		out = append(out, ToolInfo{Name: t.Name, Description: t.Description, ReadOnly: t.ReadOnly})
	}
	w.Header().Set("Content-Type", "application/json")
	if out == nil {
		w.Write([]byte("[]"))
		return
	}
	json.NewEncoder(w).Encode(out)
}

// extractSessionTitle 从 checkpoint 中提取第一条用户消息作为标题。
func extractSessionTitle(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "未知会话"
	}
	var ckpt struct {
		MessagesJSON string `json:"messages_json"`
	}
	if err := json.Unmarshal(data, &ckpt); err != nil {
		return "未知会话"
	}
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(ckpt.MessagesJSON), &msgs); err != nil {
		return "未知会话"
	}
	for _, m := range msgs {
		if m.Role == "user" {
			// 取前 24 个字
			runes := []rune(strings.TrimSpace(m.Content))
			if len(runes) > 24 {
				return string(runes[:24]) + "…"
			}
			return string(runes)
		}
	}
	return "空会话"
}

// handleStatic 提供静态文件（前端 HTML）。
// V1 返回一个简单的占位页面，后续替换为完整前端。
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}
