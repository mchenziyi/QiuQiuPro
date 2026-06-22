// QiuQiuPro UI 设计稿 — 主布局
const page = figma.currentPage;
page.name = "QiuQiuPro UI";

// 深色主题色
const bg = {r: 0.12, g: 0.12, b: 0.13};
const bgDark = {r: 0.08, g: 0.08, b: 0.09};
const border = {r: 0.18, g: 0.18, b: 0.19};
const textPrimary = {r: 0.88, g: 0.88, b: 0.88};
const textSecondary = {r: 0.55, g: 0.55, b: 0.56};
const accent = {r: 0.2, g: 0.5, b: 0.95};
const green = {r: 0.18, g: 0.6, b: 0.28};
const red = {r: 0.78, g: 0.2, b: 0.18};
const yellow = {r: 0.8, g: 0.6, b: 0.1};
const surface = {r: 0.15, g: 0.15, b: 0.16};

function div(name, x, y, w, h, fill) {
  const f = figma.createFrame();
  f.resize(w, h);
  f.name = name;
  f.x = x; f.y = y;
  f.fills = [{type: "SOLID", color: fill}];
  return f;
}

function text(content, size, color, bold) {
  const t = figma.createText();
  t.characters = content;
  t.fontSize = size;
  t.fills = [{type: "SOLID", color: color || textPrimary}];
  return t;
}

// ============ 主框架 ============
const main = div("QiuQiuPro", 0, 0, 1440, 900, bg);

// ============ Status Bar ============
const sb = div("Status Bar", 0, 0, 1440, 36, bgDark);
sb.strokes = [{type: "SOLID", color: border}];
sb.strokeWeight = 1;
main.appendChild(sb);

const logo = text("QiuQiuPro", 13, accent, true);
logo.x = 12; logo.y = 8;
sb.appendChild(logo);

const modeT = text("mode: plan", 11, textSecondary);
modeT.x = 100; modeT.y = 10;
sb.appendChild(modeT);

const skillT = text("skill: default", 11, textSecondary);
skillT.x = 190; skillT.y = 10;
sb.appendChild(skillT);

const sessionT = text("session: ses_xxxx", 11, textSecondary);
sessionT.x = 310; sessionT.y = 10;
sb.appendChild(sessionT);

const usageT = text("tokens: 1.2k↑ 0.3k↓", 11, textSecondary);
usageT.x = 460; usageT.y = 10;
sb.appendChild(usageT);

const cacheT = text("cache: 85%", 11, green, true);
cacheT.x = 620; cacheT.y = 10;
sb.appendChild(cacheT);

const statusDot = div("Running", 0, 0, 6, 6, green);
statusDot.x = 720; statusDot.y = 14;
sb.appendChild(statusDot);
const statusL = text("running", 11, green);
statusL.x = 730; statusL.y = 10;
sb.appendChild(statusL);

// ============ Sidebar (260px) ============
const side = div("Sidebar", 0, 36, 260, 864, bgDark);
side.strokes = [{type: "SOLID", color: border}];
side.strokeWeight = 1;
main.appendChild(side);

// Sidebar top: +New Session button
const newBtn = div("+ 新会话", 12, 12, 236, 32, surface);
newBtn.cornerRadius = 6;
side.appendChild(newBtn);
const newBtnT = text("+ 新会话", 12, textPrimary);
newBtnT.x = 20; newBtnT.y = 20;
newBtn.appendChild(newBtnT);

// 今天分组
const todayT = text("今天", 10, textSecondary);
todayT.x = 12; todayT.y = 60;
side.appendChild(todayT);

const sessions = [
  {name: "优化缓存策略", active: true},
  {name: "修复 MCP 子进程泄漏", active: false},
  {name: "添加 Plan 模式保护", active: false},
];
sessions.forEach((s, i) => {
  const row = div(s.name, 0, 78 + i*36, 260, 34, s.active ? surface : bgDark);
  row.name = "会话: " + s.name;
  side.appendChild(row);
  const indicator = div("indicator", 0, 78 + i*36, 3, 34, s.active ? accent : bgDark);
  side.appendChild(indicator);
  const t = text(s.name, 12, s.active ? accent : textPrimary);
  t.x = 18; t.y = 86 + i*36;
  side.appendChild(t);
});

// 昨天分组
const yesterdayT = text("昨天", 10, textSecondary);
yesterdayT.x = 12; yesterdayT.y = 210;
side.appendChild(yesterdayT);

const oldSessions = [
  {name: "跨平台兼容修复"},
  {name: "初始化项目结构"},
];
oldSessions.forEach((s, i) => {
  const row = div(s.name, 0, 228 + i*36, 260, 34, bgDark);
  row.name = "会话: " + s.name;
  side.appendChild(row);
  const t = text(s.name, 12, textPrimary);
  t.x = 18; t.y = 236 + i*36;
  side.appendChild(t);
});

// ============ Thread (820px middle) ============
const thread = div("Thread", 260, 36, 820, 864, bg);

// Thread top bar
const threadBar = div("Thread Bar", 0, 0, 820, 1, border);
thread.appendChild(threadBar);

// === Message rows ===
// User message
const userRow = div("User Message", 20, 20, 780, 30, bg);
const userT = text("> 帮我优化一下项目的缓存策略", 13, textPrimary);
userT.x = 0; userT.y = 0;
userRow.appendChild(userT);
thread.appendChild(userRow);

// Thinking folded
const thinkRow = div("Thinking", 20, 65, 780, 28, surface);
thinkRow.cornerRadius = 6;
const thinkIcon = text("▶", 11, textSecondary);
thinkIcon.x = 10; thinkIcon.y = 6;
thinkRow.appendChild(thinkIcon);
const thinkT = text("分析当前代码中的缓存机制…", 12, textSecondary);
thinkT.x = 28; thinkT.y = 5;
thinkRow.appendChild(thinkT);
thread.appendChild(thinkRow);

// Assistant message
const asstRow = div("Assistant", 20, 105, 780, 80, surface);
asstRow.cornerRadius = 6;
const asstT = text("当前项目的缓存机制主要依赖 system prompt 稳定性……", 12, textPrimary);
asstT.x = 12; asstT.y = 8;
asstT.resize(750, 20);
asstRow.appendChild(asstT);
thread.appendChild(asstRow);

// Tool: read_file
const tool1 = div("read_file", 20, 200, 780, 32, surface);
tool1.cornerRadius = 6;
const t1Icon = div("icon", 0, 0, 32, 32, {r: 0.18, g: 0.45, b: 0.85});
t1Icon.cornerRadius = 4;
tool1.appendChild(t1Icon);
const t1T = text("read_file  agent/compact.go", 12, textPrimary);
t1T.x = 42; t1T.y = 8;
tool1.appendChild(t1T);
const t1Status = text("✓", 13, green);
t1Status.x = 740; t1Status.y = 7;
tool1.appendChild(t1Status);
thread.appendChild(tool1);

// Tool: edit_file with diff summary
const tool2 = div("edit_file", 20, 245, 780, 32, surface);
tool2.cornerRadius = 6;
const t2T = text("edit_file  agent/compact.go  •  2 hunks  +15/-8", 12, textPrimary);
t2T.x = 12; t2T.y = 8;
tool2.appendChild(t2T);
const t2Status = text("✓", 13, green);
t2Status.x = 740; t2Status.y = 7;
tool2.appendChild(t2Status);
thread.appendChild(tool2);

// Usage row
const usageRow = div("Usage", 20, 295, 780, 24, bg);
const usageL = text("↑1,245  ↓387  cache 85.2%", 10, textSecondary);
usageL.x = 0; usageL.y = 0;
usageRow.appendChild(usageL);
thread.appendChild(usageRow);

// ============ Composer (bottom, 820px wide) ============
const composer = div("Composer", 260, 840, 820, 60, {r: 0.1, g: 0.1, b: 0.11});
composer.cornerRadius = 8;
composer.strokes = [{type: "SOLID", color: {r: 0.22, g: 0.22, b: 0.24}}];
composer.strokeWeight = 1;
main.appendChild(composer);

const inputArea = text("输入消息或 /command...", 12, textSecondary);
inputArea.x = 16; inputArea.y = 20;
composer.appendChild(inputArea);

const sendBtn = div("发送", 750, 14, 56, 32, accent);
sendBtn.cornerRadius = 6;
composer.appendChild(sendBtn);
const sendT = text("发送", 12, {r: 1, g: 1, b: 1});
sendT.x = 762; sendT.y = 22;
sendBtn.appendChild(sendT);

// ============ Inspector (360px right) ============
const insp = div("Inspector", 1080, 36, 360, 864, bgDark);
insp.strokes = [{type: "SOLID", color: border}];
insp.strokeWeight = 1;
main.appendChild(insp);

// Inspector header
const inspHeader = text("Diff 详情", 13, textPrimary, true);
inspHeader.x = 16; inspHeader.y = 16;
insp.appendChild(inspHeader);

// File path
const filePath = text("agent/compact.go", 11, accent);
filePath.x = 16; filePath.y = 42;
insp.appendChild(filePath);

// Diff hunks
const hunkBg = div("Hunk", 16, 66, 328, 200, {r: 0.1, g: 0.1, b: 0.11});
hunkBg.cornerRadius = 4;
insp.appendChild(hunkBg);

const ctx1 = text("  const (", 11, {r: 0.5, g: 0.5, b: 0.5});
ctx1.x = 24; ctx1.y = 74;
insp.appendChild(ctx1);

const del1 = text("−  defaultCompactRatio = 0.8", 11, red);
del1.x = 24; del1.y = 92;
insp.appendChild(del1);

const del2 = text("−  defaultSoftRatio    = 0.5", 11, red);
del2.x = 24; del2.y = 110;
insp.appendChild(del2);

const add1 = text("+  defaultCompactRatio = 0.95", 11, green);
add1.x = 24; add1.y = 130;
insp.appendChild(add1);

const add2 = text("+  defaultSoftRatio    = 0.85", 11, green);
add2.x = 24; add2.y = 148;
insp.appendChild(add2);

const ctx2 = text("  )", 11, {r: 0.5, g: 0.5, b: 0.5});
ctx2.x = 24; ctx2.y = 166;
insp.appendChild(ctx2);

// Approve / Reject buttons
const approveBtn = div("Approve", 16, 290, 100, 32, green);
approveBtn.cornerRadius = 6;
insp.appendChild(approveBtn);
const approveT = text("Approve", 11, {r: 1, g: 1, b: 1});
approveT.x = 30; approveT.y = 298;
approveBtn.appendChild(approveT);

const rejectBtn = div("Reject", 124, 290, 100, 32, {r: 0.3, g: 0.12, b: 0.1});
rejectBtn.cornerRadius = 6;
insp.appendChild(rejectBtn);
const rejectT = text("Reject", 11, textSecondary);
rejectT.x = 140; rejectT.y = 298;
rejectBtn.appendChild(rejectT);

// ============ Export as PNG ============
console.log("UI layout created successfully");
