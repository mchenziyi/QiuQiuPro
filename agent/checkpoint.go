package agent

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
