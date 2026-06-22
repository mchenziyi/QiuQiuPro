#!/usr/bin/env python3
"""Build knowledge-graph.json from nodes + edges."""
import json, os
from datetime import datetime, timezone

root = r"D:\QiuQiu\projects\QiuQiuPro"
nodes = json.load(open(os.path.join(root, ".understand-anything/intermediate/all-nodes.json")))

edges = [
  # main.go -> agent + tool + command
  {"source": "file:main.go", "target": "file:agent.go", "type": "imports", "direction": "forward", "weight": 0.9},
  {"source": "main.go", "target": "file:agent.go", "type": "imports", "direction": "forward", "weight": 0.9},
  {"source": "file:main.go", "target": "file:command/registry.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:main.go", "target": "file:tool/struct.go", "type": "imports", "direction": "forward", "weight": 0.8},
  {"source": "file:main.go", "target": "file:event/store.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:main.go", "target": "file:skill/manager.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:main.go", "target": "file:mcp/manager.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # agent/core -> agent modules
  {"source": "file:agent.go", "target": "file:agent/run.go", "type": "contains", "direction": "forward", "weight": 0.9},
  {"source": "file:agent.go", "target": "file:agent/tools.go", "type": "contains", "direction": "forward", "weight": 0.9},
  {"source": "file:agent.go", "target": "file:agent/gate.go", "type": "contains", "direction": "forward", "weight": 0.8},
  {"source": "file:agent.go", "target": "file:agent/sink.go", "type": "contains", "direction": "forward", "weight": 0.8},
  {"source": "file:agent.go", "target": "file:agent/plan.go", "type": "contains", "direction": "forward", "weight": 0.8},
  {"source": "file:agent.go", "target": "file:agent/compact.go", "type": "contains", "direction": "forward", "weight": 0.8},
  {"source": "file:agent.go", "target": "file:agent/session.go", "type": "contains", "direction": "forward", "weight": 0.8},
  {"source": "file:agent.go", "target": "file:agent/input.go", "type": "contains", "direction": "forward", "weight": 0.7},
  {"source": "file:agent.go", "target": "file:agent/skill.go", "type": "contains", "direction": "forward", "weight": 0.7},
  {"source": "file:agent.go", "target": "file:agent/prompt.go", "type": "contains", "direction": "forward", "weight": 0.7},
  {"source": "file:agent.go", "target": "file:agent/cache_shape.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/execution_state.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/prune.go", "type": "contains", "direction": "forward", "weight": 0.7},
  {"source": "file:agent.go", "target": "file:agent/long_memory.go", "type": "contains", "direction": "forward", "weight": 0.7},
  {"source": "file:agent.go", "target": "file:agent/memory_cache.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/usage.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/hooks.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/helpers.go", "type": "contains", "direction": "forward", "weight": 0.5},
  {"source": "file:agent.go", "target": "file:agent/install_tools.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/deepseek.go", "type": "contains", "direction": "forward", "weight": 0.6},
  {"source": "file:agent.go", "target": "file:agent/checkpoint.go", "type": "contains", "direction": "forward", "weight": 0.7},
  # run.go -> dependencies
  {"source": "file:agent/run.go", "target": "file:agent/session.go", "type": "uses", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/run.go", "target": "file:agent/tools.go", "type": "uses", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/run.go", "target": "file:agent/compact.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/run.go", "target": "file:agent/sink.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/run.go", "target": "file:agent/gate.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/run.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/run.go", "target": "file:agent/skill.go", "type": "uses", "direction": "forward", "weight": 0.6},
  {"source": "file:agent/run.go", "target": "file:agent/long_memory.go", "type": "uses", "direction": "forward", "weight": 0.6},
  {"source": "file:agent/run.go", "target": "file:agent/plan.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/run.go", "target": "file:agent/input.go", "type": "uses", "direction": "forward", "weight": 0.6},
  {"source": "file:agent/run.go", "target": "file:agent/cache_shape.go", "type": "uses", "direction": "forward", "weight": 0.5},
  {"source": "file:agent/run.go", "target": "file:agent/execution_state.go", "type": "uses", "direction": "forward", "weight": 0.5},
  {"source": "file:agent/run.go", "target": "file:agent/hooks.go", "type": "uses", "direction": "forward", "weight": 0.5},
  # tools.go -> tool packages
  {"source": "file:agent/tools.go", "target": "file:tool/struct.go", "type": "imports", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/tools.go", "target": "file:tool/all_tools.go", "type": "imports", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/tools.go", "target": "file:skill/skill.go", "type": "imports", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/tools.go", "target": "file:skill/manager.go", "type": "imports", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/tools.go", "target": "file:mcp/manager.go", "type": "imports", "direction": "forward", "weight": 0.6},
  # plan.go
  {"source": "file:agent/plan.go", "target": "file:agent/session.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/plan.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/plan.go", "target": "file:agent/sink.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # compact.go
  {"source": "file:agent/compact.go", "target": "file:agent/session.go", "type": "uses", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/compact.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.7},
  {"source": "file:agent/compact.go", "target": "file:agent/prune.go", "type": "uses", "direction": "forward", "weight": 0.8},
  # session.go
  {"source": "file:agent/session.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/session.go", "target": "file:agent/usage.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # checkpoint.go
  {"source": "file:agent/checkpoint.go", "target": "file:agent/session.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/checkpoint.go", "target": "file:event/store.go", "type": "uses", "direction": "forward", "weight": 0.8},
  # gate.go
  {"source": "file:agent/gate.go", "target": "file:tool/struct.go", "type": "imports", "direction": "forward", "weight": 0.7},
  # sink.go
  {"source": "file:agent/sink.go", "target": "file:event/store.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # prompt.go
  {"source": "file:agent/prompt.go", "target": "file:prompts_xml", "type": "embeds", "direction": "forward", "weight": 0.9},
  # skill
  {"source": "file:agent/skill.go", "target": "file:skill/skill.go", "type": "imports", "direction": "forward", "weight": 0.9},
  {"source": "file:agent/skill.go", "target": "file:skill/manager.go", "type": "uses", "direction": "forward", "weight": 0.8},
  # long_memory.go
  {"source": "file:agent/long_memory.go", "target": "file:event/store.go", "type": "uses", "direction": "forward", "weight": 0.8},
  # memory_cache.go
  {"source": "file:agent/memory_cache.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.6},
  {"source": "file:agent/memory_cache.go", "target": "file:qiuqiu_global_template", "type": "embeds", "direction": "forward", "weight": 0.8},
  {"source": "file:agent/memory_cache.go", "target": "file:qiuqiu_project_template", "type": "embeds", "direction": "forward", "weight": 0.8},
  # tool/all_tools.go
  {"source": "file:tool/all_tools.go", "target": "file:tool/struct.go", "type": "implements", "direction": "forward", "weight": 0.9},
  # mcp
  {"source": "file:mcp/manager.go", "target": "file:mcp/client.go", "type": "contains", "direction": "forward", "weight": 0.9},
  {"source": "file:mcp/manager.go", "target": "file:agent/tools.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:mcp/client.go", "target": "file:tool/struct.go", "type": "imports", "direction": "forward", "weight": 0.7},
  # event/store.go
  {"source": "file:event/store.go", "target": "file:agent/sink.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # command/registry.go
  {"source": "file:command/registry.go", "target": "file:agent/agent.go", "type": "uses", "direction": "forward", "weight": 0.8},
  {"source": "file:command/registry.go", "target": "file:agent/prompt.go", "type": "uses", "direction": "forward", "weight": 0.6},
  # cleanup
  {"source": "file:cleanup/cleanup.go", "target": "file:tool/struct.go", "type": "implements", "direction": "forward", "weight": 0.8},
  {"source": "file:cleanup/cleanup.go", "target": "file:command/registry.go", "type": "registers", "direction": "forward", "weight": 0.7},
  # storm_test.go
  {"source": "file:agent/storm_test.go", "target": "file:agent/run.go", "type": "tests", "direction": "forward", "weight": 0.7},
  # docs
  {"source": "file:docs", "target": "file:STRUCTURES.md", "type": "documents", "direction": "forward", "weight": 0.5},
  {"source": "file:docs", "target": "file:OPTIMIZATION_SUMMARY.md", "type": "documents", "direction": "forward", "weight": 0.5},
  {"source": "file:docs", "target": "file:README.md", "type": "documents", "direction": "forward", "weight": 0.5},
  {"source": "file:docs", "target": "file:TODO.md", "type": "documents", "direction": "forward", "weight": 0.5},
  {"source": "file:docs", "target": "file:e2e_results", "type": "documents", "direction": "forward", "weight": 0.5},
  # reasonix
  {"source": "file:reasonix_data", "target": "file:reasonix.toml", "type": "configures", "direction": "forward", "weight": 0.6},
  # go.mod
  {"source": "file:go.mod", "target": "file:agent/deepseek.go", "type": "defines", "direction": "forward", "weight": 0.7},
  {"source": "file:go.mod", "target": "file:mcp/client.go", "type": "defines", "direction": "forward", "weight": 0.6},
  {"source": "file:go.mod", "target": "file:tool/all_tools.go", "type": "defines", "direction": "forward", "weight": 0.6},
  {"source": "file:go.mod", "target": "file:cleanup/cleanup.go", "type": "defines", "direction": "forward", "weight": 0.5},
]

# Fix duplicate/invalid edge ids - use proper file: prefix
def fix_edge(e):
    for k in ("source", "target"):
        v = e[k]
        if v == "main.go":
            e[k] = "file:main.go"
        elif v == "prompts_xml" and e["type"] == "embeds":
            e[k] = "file:prompts_xml"
        elif v == "e2e_results":
            e[k] = "file:e2e_results"
        elif v.startswith("file:") or v.startswith("func:") or v.startswith("config:"):
            pass
        else:
            e[k] = f"file:{v}"
    return e

edges = [fix_edge(e) for e in edges]

# Build knowledge graph
kg = {
    "version": "1.0.0",
    "kind": "knowledge",
    "project": {
        "name": "QiuQiuPro",
        "description": "从零手写 Agent 系统的实战产物，基于 Go 实现，支持 DeepSeek V4、MCP、Skill 人格、上下文压缩、规划/反思等完整 Agent 能力",
        "languages": ["go", "markdown", "json", "toml"],
        "frameworks": ["openai-go", "mcp-go"],
        "analyzedAt": datetime.now(timezone.utc).isoformat(),
        "gitCommitHash": "unknown"
    },
    "nodes": nodes,
    "edges": edges,
    "layers": [
        {
            "id": "layer-entry",
            "name": "Entry Point",
            "description": "程序入口和 CLI 层",
            "nodeIds": ["file:main.go", "file:go.mod", "file:command/registry.go"]
        },
        {
            "id": "layer-agent",
            "name": "Agent Core",
            "description": "Agent 核心：循环、会话、工具分发、规划、压缩",
            "nodeIds": ["file:agent.go", "file:agent/run.go", "file:agent/tools.go", "file:agent/session.go", "file:agent/plan.go", "file:agent/compact.go", "file:agent/gate.go", "file:agent/sink.go", "file:agent/input.go", "file:agent/prompt.go", "file:agent/skill.go", "file:agent/long_memory.go", "file:agent/memory_cache.go", "file:agent/cache_shape.go", "file:agent/execution_state.go", "file:agent/prune.go", "file:agent/hooks.go", "file:agent/helpers.go", "file:agent/install_tools.go", "file:agent/deepseek.go", "file:agent/checkpoint.go", "file:agent/usage.go", "file:agent/storm_test.go"]
        },
        {
            "id": "layer-tool",
            "name": "Tool Layer",
            "description": "工具实现层：文件操作、搜索、Shell、Git、Web 等",
            "nodeIds": ["file:tool/struct.go", "file:tool/all_tools.go", "file:cleanup/cleanup.go"]
        },
        {
            "id": "layer-external",
            "name": "External Integration",
            "description": "外部集成：MCP、Skill 管理器",
            "nodeIds": ["file:mcp/manager.go", "file:mcp/client.go", "file:skill/skill.go", "file:skill/manager.go"]
        },
        {
            "id": "layer-storage",
            "name": "Storage Layer",
            "description": "持久化层：事件存储、检查点",
            "nodeIds": ["file:event/store.go", "file:agent/checkpoint.go"]
        },
        {
            "id": "layer-docs",
            "name": "Documentation",
            "description": "文档和参考资料",
            "nodeIds": ["file:docs", "file:STRUCTURES.md", "file:OPTIMIZATION_SUMMARY.md", "file:README.md", "file:TODO.md", "file:e2e_results", "file:reasonix_data", "file:reasonix.toml", "file:prompts_xml", "file:qiuqiu_global_template", "file:qiuqiu_project_template", "file:run_e2e_self_sh"]
        }
    ],
    "tour": [
        {"order": 1, "title": "入口", "description": "从 main.go 开始，了解程序如何启动", "nodeIds": ["file:main.go"]},
        {"order": 2, "title": "Agent 核心", "description": "理解 Agent 的核心循环和工具分发机制", "nodeIds": ["file:agent.go", "file:agent/run.go", "file:agent/tools.go"]},
        {"order": 3, "title": "会话管理", "description": "学习会话历史和上下文管理", "nodeIds": ["file:agent/session.go", "file:agent/compact.go"]},
        {"order": 4, "title": "工具层", "description": "探索所有内置工具和 MCP 集成", "nodeIds": ["file:tool/struct.go", "file:tool/all_tools.go", "file:mcp/manager.go"]},
        {"order": 5, "title": "规划与反思", "description": "了解 Agent 的规划、反思和重规划能力", "nodeIds": ["file:agent/plan.go"]},
        {"order": 6, "title": "长期记忆", "description": "探索 Skill 人格和长期记忆系统", "nodeIds": ["file:agent/skill.go", "file:agent/long_memory.go"]},
        {"order": 7, "title": "文档", "description": "查看开发文档和参考资料", "nodeIds": ["file:docs"]}
    ]
}

out = os.path.join(root, ".understand-anything", "knowledge-graph.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump(kg, f, indent=2, ensure_ascii=False)
print(f"Wrote {len(nodes)} nodes, {len(edges)} edges to {out}")
