import json, urllib.request

url = "http://127.0.0.1:5182/knowledge-graph.json?token=testtoken12345"
with urllib.request.urlopen(url) as f:
    served = json.loads(f.read())

# Count node types
from collections import Counter
node_types = Counter(n.get('type') for n in served['nodes'])
edge_types = Counter(e.get('type') for e in served['edges'])

print("=== Node Types ===")
for t, c in node_types.most_common():
    print(f"  {t}: {c}")

print("\n=== Edge Types ===")
for t, c in edge_types.most_common():
    print(f"  {t}: {c}")

print(f"\nTotal nodes: {len(served['nodes'])}")
print(f"Total edges: {len(served['edges'])}")
