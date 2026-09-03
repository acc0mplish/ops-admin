#!/usr/bin/env python3
from pathlib import Path
import re
import sys

path = Path(__file__).resolve().parents[2] / "web/src/views/monitor/MonitorDashboard.vue"
text = path.read_text(encoding="utf-8")
replacements = [
    ("容器 CPU 사용 Trend", "Container CPU 사용 Trend"),
    ("容器Memory 사용 Trend", "Container Memory 사용 Trend"),
    ("PromQL / 错误", "PromQL / Error"),
    ("'提示'", "'확인'"),
    ("导出Inspection Report PDF", "Inspection Report PDF Export"),
    ("<span>当前</span>", "<span>현재</span>"),
    ("실시간采样", "실시간 Sampling"),
    ("Monitoring Dashboard / 网格展示", "Monitoring Dashboard / Grid Layout"),
]
for old, new in replacements:
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
remaining = [(n, line) for n, line in enumerate(text.splitlines(), 1) if re.search(r'[\u3400-\u4DBF\u4E00-\u9FFF]', line)]
print(f"web batch12 cleanup left {len(remaining)} Han line(s)")
for n, line in remaining[:30]:
    print(f"REMAINING: {n}: {line.strip()}", file=sys.stderr)
raise SystemExit(1 if remaining else 0)
