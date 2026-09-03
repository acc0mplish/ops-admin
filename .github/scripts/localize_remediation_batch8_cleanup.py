#!/usr/bin/env python3
from pathlib import Path
import re
import sys

path = Path(__file__).resolve().parents[2] / "backend/service/ops_application.go"
text = path.read_text(encoding="utf-8")
replacements = [
    ('Kubernetes Deployment阶段「%s」缺少集群、命名空间、工作负载或容器配置', 'Kubernetes deployment stage %q is missing cluster, namespace, workload, or container configuration'),
    ('当前流水线没有 Kubernetes Deployment阶段，无法自动回滚', 'the current pipeline has no Kubernetes deployment stage and cannot be rolled back automatically'),
    ('Kubernetes Deployment需要选择目标集群', 'Kubernetes deployment requires a target cluster'),
    ('远程Source Checkout', 'Remote Source Checkout'),
    ('远程Execution Script', 'Remote Script Execution'),
    ('远程Push Image to Registry: ', 'Remote Registry Push: '),
    ('远程Post-deployment Health Check: ', 'Remote Post-deployment Health Check: '),
    ('未配置Image Registry Login凭据，跳过 docker login。\\n', 'Image registry credentials are not configured; skipping docker login.\\n'),
    ('远程Source Checkout失败', 'remote source checkout failed'),
    ('Executing构建脚本', 'Executing build script'),
    ('Executing构建后操作', 'Executing post-build operation'),
    ('Created %d delivery task(s) through notification rule #%d; review actual results in Notifications / Send Logs.\\n', 'Created delivery tasks through notification rule #%d: %d task(s); review actual results in Notifications / Send Logs.\\n'),
]
for old, new in replacements:
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
remaining = [(n, line) for n, line in enumerate(text.splitlines(), 1) if re.search(r'[\u3400-\u4DBF\u4E00-\u9FFF]', line)]
print(f"batch8 cleanup left {len(remaining)} Han line(s)")
for n, line in remaining[:20]:
    print(f"REMAINING: {n}: {line.strip()}", file=sys.stderr)
raise SystemExit(1 if remaining else 0)
