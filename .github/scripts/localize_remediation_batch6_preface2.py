#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).resolve().parents[2] / "backend/service/integration_finops.go"
lines = path.read_text(encoding="utf-8").splitlines()
updated = []
for line in lines:
    indent = line[: len(line) - len(line.lstrip())]
    stripped = line.strip()
    if stripped.startswith('description.WriteString("\\n\\n## 空闲资源'):
        line = indent + 'description.WriteString("\\n\\n## 유휴 Resource\\n중지 상태지만 비용이 발생하거나 업무 Access 또는 Monitoring Load가 없는 Resource를 우선 검증하십시오.")'
    elif stripped.startswith('description.WriteString("\\n\\n## 低利用率资源'):
        line = indent + 'description.WriteString("\\n\\n## 저활용 Resource\\nCPU, Memory, IOPS, Connection 등의 Monitoring 데이터를 함께 확인해 Downsize 여부를 결정하십시오.")'
    elif stripped.startswith('description.WriteString("\\n\\n## 计费方式优化'):
        line = indent + 'description.WriteString("\\n\\n## Billing 방식 최적화\\n안정적인 Workload에 Subscription, Savings Plan 또는 Reserved Instance가 적합한지 평가하십시오.")'
    elif stripped.startswith('description.WriteString("\\n\\n## 闲置磁盘/快照/IP'):
        line = indent + 'description.WriteString("\\n\\n## 유휴 Disk / Snapshot / IP\\n연결되지 않은 Disk, 장기 Snapshot, 미연결 IP, Backend가 없는 Load Balancer를 점검하십시오.")'
    elif stripped.startswith('description.WriteString(fmt.Sprintf("\\n\\n## 预计可节省金额'):
        line = indent + 'description.WriteString(fmt.Sprintf("\\n\\n## 예상 절감액\\n이 Report의 분석 대상 비용은 %.2f이며 AI 예상 절감액은 %.2f입니다.", total, saving))'
    updated.append(line)
path.write_text("\n".join(updated) + "\n", encoding="utf-8")
