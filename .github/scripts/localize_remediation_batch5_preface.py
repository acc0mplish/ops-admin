#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).resolve().parents[2] / "backend/service/k8s.go"
text = path.read_text(encoding="utf-8")
replacements = {
    "// PVC 列表与详情应展示用户声明的申请容量；绑定后 status.capacity": "// PVC lists and details should display the requested capacity declared by the user; after binding, status.capacity",
    "// 表示实际绑定 PV 的容量，可能大于 PVC 请求容量。": "// represents the actual PV capacity and may be larger than the PVC request.",
    "// 部分受限集群不会返回 ownerReferences；在这种情况下按 Kubernetes": "// Some restricted clusters do not return ownerReferences. In that case, use Kubernetes",
    "// 控制器生成的 Pod 名称前缀兜底，并优先选择最长的工作负载名称。": "// controller-generated Pod name prefixes as a fallback and prefer the longest matching workload name.",
}
for old, new in replacements.items():
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
