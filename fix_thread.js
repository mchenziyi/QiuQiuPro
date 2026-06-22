// 重新创建 Thread 并放入 QiuQiuPro 主框架
const main = figma.currentPage.findOne(n => n.name === "QiuQiuPro");
if (!main) { console.log("main not found"); process.exit(1); }

const bg = {r: 0.12, g: 0.12, b: 0.13};
const surface = {r: 0.15, g: 0.15, b: 0.16};
const border = {r: 0.18, g: 0.18, b: 0.18};
const textPrimary = {r: 0.88, g: 0.88, b: 0.88};
const textSecondary = {r: 0.55, g: 0.55, b: 0.56};
const accent = {r: 0.2, g: 0.5, b: 0.95};
const green = {r: 0.18, g: 0.6, b: 0.28};

function frame(name, x, y, w, h, fill) {
  const f = figma.createFrame();
  f.resize(w, h);
  f.name = name; f.x = x; f.y = y;
  f.fills = [{type: "SOLID", color: fill}];
  return f;
}
function txt(content, size, color) {
  const t = figma.createText();
  t.characters = content; t.fontSize = size;
  t.fills = [{type: "SOLID", color: color || textPrimary}];
  return t;
}

const thread = frame("Thread", 260, 40, 820, 800, bg);
const bar = frame("Thread Bar", 0, 0, 820, 1, border);
thread.appendChild(bar);

const userRow = frame("User Message", 20, 20, 780, 24, bg);
const ut = txt("> 帮我优化一下项目的缓存策略", 13, textPrimary); ut.x = 0; ut.y = 0;
userRow.appendChild(ut);
thread.appendChild(userRow);

const thinkRow = frame("Thinking", 20, 58, 780, 28, surface);
thinkRow.cornerRadius = 6;
const tt = txt("▶ 分析当前代码中的缓存机制…", 12, textSecondary); tt.x = 10; tt.y = 6;
thinkRow.appendChild(tt);
thread.appendChild(thinkRow);

const asstRow = frame("Assistant", 20, 100, 780, 70, surface);
asstRow.cornerRadius = 6;
const at = txt("当前项目的缓存机制主要依赖 system prompt 稳定性……", 12, textPrimary); at.x = 12; at.y = 8;
asstRow.appendChild(at);
thread.appendChild(asstRow);

const tool1 = frame("read_file", 20, 185, 780, 32, surface);
tool1.cornerRadius = 6;
const ic = frame("icon", 0, 0, 32, 32, {r: 0.18, g: 0.45, b: 0.85}); ic.cornerRadius = 4;
tool1.appendChild(ic);
const t1t = txt("read_file  agent/compact.go", 12, textPrimary); t1t.x = 42; t1t.y = 8;
tool1.appendChild(t1t);
const t1s = txt("✓", 13, green); t1s.x = 740; t1s.y = 6;
tool1.appendChild(t1s);
thread.appendChild(tool1);

const tool2 = frame("edit_file", 20, 228, 780, 32, surface);
tool2.cornerRadius = 6;
const t2t = txt("edit_file  agent/compact.go  •  2 hunks  +15/-8", 12, textPrimary); t2t.x = 12; t2t.y = 8;
tool2.appendChild(t2t);
const t2s = txt("✓", 13, green); t2s.x = 740; t2s.y = 6;
tool2.appendChild(t2s);
thread.appendChild(tool2);

const usageRow = frame("Usage", 20, 275, 780, 20, bg);
const ult = txt("↑1,245  ↓387  cache 85.2%", 10, textSecondary); ult.x = 0; ult.y = 0;
usageRow.appendChild(ult);
thread.appendChild(usageRow);

main.appendChild(thread);
console.log("Thread recreated with " + thread.children.length + " children");
