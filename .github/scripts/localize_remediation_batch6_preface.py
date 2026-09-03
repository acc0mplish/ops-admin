#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def replace_lines(path: Path, transform):
    lines = path.read_text(encoding="utf-8").splitlines()
    updated = [transform(line) for line in lines]
    path.write_text("\n".join(updated) + "\n", encoding="utf-8")


def finops_line(line: str) -> str:
    indent = line[: len(line) - len(line.lstrip())]
    stripped = line.strip()
    if stripped.startswith('description.WriteString("## 执行摘要') and '默认 FinOps' in stripped:
        return indent + 'description.WriteString("## 실행 요약\\n이번 달 Billing을 기준으로 생성한 기본 FinOps 분석 결과입니다. Billing에는 CPU, Memory, Connection 같은 실시간 Monitoring Metric이 없으므로 유휴 및 저활용 항목은 검증 대상입니다.")'
    if stripped.startswith('description.WriteString(fmt.Sprintf("\\n%d. %s：本月成本'):
        return indent + 'description.WriteString(fmt.Sprintf("\\n%d. %s: 이번 달 비용 %.2f. 사용률, 유휴 시간대, 약정 할인 적용 가능성을 검토하십시오. 예상 절감액은 %.2f입니다.", index+1, name, item.Cost, recommendationSaving))'
    if stripped.startswith('description.WriteString("\\n\\n## 空闲资源') and '对应运行负载' in stripped:
        return indent + 'description.WriteString("\\n\\n## 유휴 Resource\\n이번 달에도 비용이 발생하지만 대응하는 Runtime Workload 또는 업무 Access가 없는 Resource를 우선 확인하십시오. 검증 후 중지, 해제 또는 Scheduled Start/Stop을 적용할 수 있습니다.")'
    if stripped.startswith('description.WriteString("\\n\\n## 低利用率资源') and '上述高成本' in stripped:
        return indent + 'description.WriteString("\\n\\n## 저활용 Resource\\n고비용 Compute, Database, Middleware Resource의 CPU, Memory, Connection, IOPS를 확인하십시오. 지속적으로 사용률이 낮으면 Downsize를 검토합니다.")'
    if stripped.startswith('description.WriteString("\\n\\n## 计费方式优化') and '稳定运行资源' in stripped:
        return indent + 'description.WriteString("\\n\\n## Billing 방식 최적화\\n안정적으로 실행되는 Resource에는 Subscription, Savings Plan 또는 Reserved Instance를 평가하고 중복 Resource Package 구매를 방지하십시오.")'
    if stripped.startswith('description.WriteString("\\n\\n## 闲置磁盘/快照/IP') and '盘点未挂载' in stripped:
        return indent + 'description.WriteString("\\n\\n## 유휴 Disk / Snapshot / IP\\n연결되지 않은 Cloud Disk, 장기 Snapshot, 미연결 EIP, 유휴 Load Balancer를 점검하고 Dependency가 없음을 확인한 뒤 정리하십시오.")'
    if stripped.startswith('description.WriteString(fmt.Sprintf("\\n\\n## 预计可节省金额') and '保守 15' in stripped:
        return indent + 'description.WriteString(fmt.Sprintf("\\n\\n## 예상 절감액\\n이번 달 분석 대상 비용은 %.2f이며 보수적인 15%% 기준 예상 월 절감액은 %.2f입니다.", total, saving))'
    if stripped.startswith('recommendation.Description = "## AI 分析结论'):
        return indent + 'recommendation.Description = "## AI 분석 결론\\n" + truncateRunes(content, 12000)'
    if stripped.startswith('prompt := "根据以下云费用分析工具结果'):
        return indent + 'prompt := "Using the following cloud-cost analysis tool result, generate no more than five actionable Korean optimization recommendations. The tool result comes only from the locally synchronized billing database and does not represent real-time cloud state. Cover idle resources, low utilization, billing-model optimization, idle disks/snapshots/IPs, and estimated savings. When billing lacks real-time monitoring metrics, state that validation is required rather than claiming a resource is idle. Each description must be no longer than 80 Korean characters. Return exactly one complete JSON object: {\\"recommendations\\":[{\\"accountId\\":1,\\"provider\\":\\"...\\",\\"resourceId\\":\\"...\\",\\"priority\\":\\"P1|P2|P3\\",\\"title\\":\\"...\\",\\"description\\":\\"...\\",\\"currentCost\\":0,\\"saving\\":0}]}. Do not output Markdown fences, headings, or text outside JSON. Do not invent resources; saving must be non-negative and no greater than currentCost. Data: " + string(contextJSON)'
    if stripped.startswith('repairPrompt := "将以下 FinOps 分析结果转换'):
        return indent + 'repairPrompt := "Convert the following FinOps analysis result into one complete JSON object. Return JSON only, without Markdown or explanation. The schema must be {\\"recommendations\\":[{\\"accountId\\":1,\\"provider\\":\\"...\\",\\"resourceId\\":\\"...\\",\\"priority\\":\\"P1|P2|P3\\",\\"title\\":\\"...\\",\\"description\\":\\"...\\",\\"currentCost\\":0,\\"saving\\":0}]}. If fields are missing, use conservative values from the known account and cost data and do not invent resources. Original content:\\n" + truncateRunes(response.Content, 12000)'
    if stripped.startswith('description.WriteString("## 执行摘要') and 'AI 基于本月账单' in stripped:
        return indent + 'description.WriteString("## 실행 요약\\n다음은 AI가 이번 달 Billing을 기준으로 생성한 종합 최적화 권고입니다.")'
    if stripped.startswith('description.WriteString(fmt.Sprintf("\\n%d. %s：%s'):
        return indent + 'description.WriteString(fmt.Sprintf("\\n%d. %s: %s (현재 비용 %.2f, 예상 절감액 %.2f)", index+1, item.Title, item.Description, item.CurrentCost, item.Saving))'
    return line


def notify_line(line: str) -> str:
    indent = line[: len(line) - len(line.lstrip())]
    stripped = line.strip()
    if stripped.startswith('return "【定时任务】'):
        return indent + 'return "[Scheduled Task] {{taskName}} · {{status}}", "**실행 상태:** {{status}}\\n\\n**Task 이름:** {{taskName}}\\n**Task Type:** {{taskType}}\\n**Trigger 방식:** {{triggerType}}\\n**Cron:** {{cronExpr}}\\n**실행 시간:** {{duration}}\\n**완료 시각:** {{finishedAt}}\\n\\n---\\n\\n**실행 요약**\\n{{summary}}\\n\\n{{detail}}"'
    if stripped.startswith('return "【流水线通知】'):
        return indent + 'return "[Pipeline Notification] {{pipelineName}} · {{stageName}}", "**실행 상태:** {{status}}\\n\\n**Pipeline:** {{pipelineName}}\\n**Run ID:** #{{pipelineRunId}}\\n**Application:** {{appName}}\\n**Environment:** {{env}}\\n**Branch:** {{branch}}\\n**Image Version:** {{imageTag}}\\n**알림 시각:** {{notifyAt}}\\n\\n---\\n\\n**실행 요약**\\n{{summary}}\\n\\n{{detail}}"'
    if stripped.startswith('return "【作业通知】'):
        return indent + 'return "[Job Notification] {{jobName}} · {{stepName}}", "**알림 Type:** {{status}}\\n\\n**Job 이름:** {{jobName}}\\n**Run ID:** #{{jobHistoryId}}\\n**현재 Step:** {{stepName}}\\n**Trigger 방식:** {{triggerType}}\\n**알림 시각:** {{notifyAt}}\\n\\n---\\n\\n**알림 요약**\\n{{summary}}\\n\\n{{detail}}"'
    return line


replace_lines(ROOT / "backend/service/integration_finops.go", finops_line)
replace_lines(ROOT / "backend/service/notify.go", notify_line)
