package agent

import (
	"fmt"
	"time"
)

// 检查点持久化：把会话历史序列化存档 / 从存档恢复（协调 Session 与 event.Store）。

func (a *Agent) SaveCheckpoint() {
	data, _ := a.session.Snapshot()
	a.store.SaveCheckpoint(a.session.ID, a.lastEventID, data)
}

func (a *Agent) RestoreFromCheckpoint() bool {
	cp, err := a.store.LoadCheckpoint(a.session.ID)
	if err != nil || cp == nil {
		return false
	}
	if err := a.session.Restore(cp.MessagesJSON); err != nil {
		return false
	}
	a.lastEventID = cp.LastEventID
	a.noticef("  💾 从快照恢复 %d 条消息\n", a.session.Len())
	return true
}

// SwitchSession 切换到指定会话，加载其 checkpoint。返回 false 表示会话不存在。
func (a *Agent) SwitchSession(id string) bool {
	cp, err := a.store.LoadCheckpoint(id)
	if err != nil || cp == nil {
		return false
	}
	a.session = NewSession(id)
	a.lastEventID = cp.LastEventID
	if err := a.session.Restore(cp.MessagesJSON); err != nil {
		return false
	}
	a.sessCacheHit.Store(0)
	a.sessCacheMiss.Store(0)
	return true
}

// ResetSession 清空当前会话，开始新会话。
func (a *Agent) ResetSession() {
	id := fmt.Sprintf("session_%d", time.Now().Unix())
	a.session = NewSession(id)
	a.lastEventID = ""
	a.sessCacheHit.Store(0)
	a.sessCacheMiss.Store(0)
}
