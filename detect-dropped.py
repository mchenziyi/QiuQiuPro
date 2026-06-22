import json, sys

# Read original nodes from the file we wrote
with open(r"D:\QiuQiu\projects\QiuQiuPro\.understand-anything\knowledge-graph.json") as f:
    kg = json.load(f)

# Read nodes from vite served version (after normalization)
import urllib.request
url = "http://127.0.0.1:5182/knowledge-graph.json?token=testtoken12345"
with urllib.request.urlopen(url) as f:
    served = json.loads(f.read())

orig_ids = set(n["id"] for n in kg["nodes"])
served_ids = set(n["id"] for n in served["nodes"])

dropped = orig_ids - served_ids
print(f"Original nodes: {len(orig_ids)}")
print(f"Served nodes: {len(served_ids)}")
print(f"Dropped: {len(dropped)}")
print()
for nid in sorted(dropped):
    node = next(n for n in kg["nodes"] if n["id"] == nid)
    print(f"  {nid} (type={node['type']})")
