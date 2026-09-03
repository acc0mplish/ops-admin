#!/usr/bin/env python3
from pathlib import Path
import re
import sys

path = Path(__file__).resolve().parents[2] / "web/src/views/assets/DatabaseWorkbench.vue"
text = path.read_text(encoding="utf-8")
replacements = [
    ("'数据Database'", "'Database'"),
    ("`${createDatabaseObjectLabel.value} ${name} 已创建`", "`${createDatabaseObjectLabel.value} ${name}을(를) 생성했습니다.`"),
    ("`Redis Key 已${action}`", "`Redis Key ${action}을(를) 완료했습니다.`"),
    ("title=\"当前数据Database不是 SQL Type\"", "title=\"현재 Database는 SQL Type이 아닙니다.\""),
    ("<span>耗时：{{ execMeta.durationMs }} ms</span>", "<span>Duration: {{ execMeta.durationMs }} ms</span>"),
    ("label=\"导入Export Task\"", "label=\"Import / Export Task\""),
    ("row.taskType === 'export' ? '导出' : '导入'", "row.taskType === 'export' ? 'Export' : 'Import'"),
    (":label=\"`请输入“${sqlConfirmationText()}”确认`\"", ":label=\"`실행을 확인하려면 “${sqlConfirmationText()}”을(를) 입력하십시오.`\""),
    ("'当前记录没有Rollback SQL'", "'현재 Record에는 Rollback SQL이 없습니다.'"),
    ("title=\"创建Import Task\"", "title=\"Import Task 생성\""),
]
for old, new in replacements:
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
remaining = [(n, line) for n, line in enumerate(text.splitlines(), 1) if re.search(r'[\u3400-\u4DBF\u4E00-\u9FFF]', line)]
print(f"web batch11 cleanup left {len(remaining)} Han line(s)")
for n, line in remaining[:30]:
    print(f"REMAINING: {n}: {line.strip()}", file=sys.stderr)
raise SystemExit(1 if remaining else 0)
