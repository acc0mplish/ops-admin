#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[2]
files = {
    "backend/service/ops_application.go": [
        ('Remote Source Checkout失败', 'Remote Source Checkout failed'),
    ],
    "backend/service/monitor.go": [
        ('// VictoriaLogs 的 /query 和 /hits 在起始时间边界上偶发不一致：', '// VictoriaLogs /query and /hits can occasionally disagree at the start-time boundary:'),
        ('// /hits 已计入一条记录，但 /query 返回空行。仅在这个异常分支把起点', '// /hits may count a record while /query returns no rows. Only in this exceptional branch,'),
        ('// 向前补偿一秒重试，避免页面出现“命中 1 条却没有日志”的假空结果。', '// retry with the start time shifted back by one second to avoid a false empty result.'),
        ('// 不向 VictoriaLogs field_values 透传 limit。部分版本在携带该参数时仍会', '// Do not pass limit to VictoriaLogs field_values. Some versions return field values but'),
        ('// 返回字段值，但 hits 会全部变成 0；在收到带真实命中数的结果后再统一截取。', '// report every hit count as zero when the parameter is present; truncate only after receiving real counts.'),
        ('导入项的名称和query is required', 'imported item name and query are required'),
        ('All Logs数据源', 'All Log Datasources'),
        ('屏蔽rule name is required', 'silence rule name is required'),
        ('告警规则已批量停用，System自动关闭未结束事件', 'alert rules disabled in batch; open events were closed automatically'),
        ('// 恢复通知只能对应一条已经发出过触发通知的告警。pending 事件在持续', '// A recovery notification must correspond to an alert that emitted a firing notification. A pending event'),
        ('// 时间内恢复时并未对外告警，因此不能产生一条孤立的“恢复”消息。', '// that recovers during its duration window emitted no external alert and must not create an orphan recovery message.'),
    ],
}

remaining = []
for relative, replacements in files.items():
    path = root / relative
    text = path.read_text(encoding="utf-8")
    for old, new in replacements:
        text = text.replace(old, new)
    path.write_text(text, encoding="utf-8")
    remaining.extend((relative, n, line) for n, line in enumerate(text.splitlines(), 1) if re.search(r'[\u3400-\u4DBF\u4E00-\u9FFF]', line))

print(f"batch9 cleanup left {len(remaining)} Han line(s)")
for relative, n, line in remaining[:30]:
    print(f"REMAINING: {relative}:{n}: {line.strip()}", file=sys.stderr)
raise SystemExit(1 if remaining else 0)
