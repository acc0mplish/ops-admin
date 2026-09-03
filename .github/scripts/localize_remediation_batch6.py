#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/service/integration_finops.go": [
        ('"内置官方账单 API"', '"Built-in Official Billing API"'),
        ('"账单适配器"', '"Billing Adapter"'),
        ('"自定义账单适配器"', '"Custom Billing Adapter"'),
        ('"云账号名称和有效的云厂商不能为空"', '"cloud account name and a valid provider are required"'),
        ('"无效的账单同步频率"', '"invalid billing synchronization frequency"'),
        ('"云账号 ID 不能为空"', '"cloud account ID is required"'),
        ('"云账号不存在"', '"cloud account does not exist"'),
        ('fmt.Errorf("第 %d 条账单日期无效: %w", i+1, err)', 'fmt.Errorf("billing record %d has an invalid date: %w", i+1, err)'),
        ('+"账单同步完成 "+monthText+"（未返回账单明细，保留现有入库快照）"', '+" billing synchronization completed for "+monthText+"; no detail rows were returned, so the existing stored snapshot was retained"'),
        ('+"账单同步完成 "+monthText', '+" billing synchronization completed for "+monthText'),
        ('"未分类"', '"Uncategorized"'),
        ('"未关联资源|"', '"Unlinked Resource|"'),
        ('"内存 "+value', '"Memory "+value'),
        ('"磁盘 "+value', '"Disk "+value'),
        ('"month 参数格式无效，格式为 YYYY-MM"', '"invalid month parameter; expected YYYY-MM"'),
        ('"无效的建议生成策略"', '"invalid recommendation generation strategy"'),
        ('analysisScope := "全部云账号"', 'analysisScope := "전체 Cloud Account"'),
        ('scope = "全部云账号"', 'scope = "전체 Cloud Account"'),
        ('strategyName := "默认策略"', 'strategyName := "기본 Strategy"'),
        ('strategyName = "AI 分析"', 'strategyName = "AI 분석"'),
        ('strategyName = "AI 分析降级（默认策略）"', 'strategyName = "AI Fallback (기본 Strategy)"'),
        ('return fmt.Sprintf("%s｜%s｜%s优化建议", scope, analysisMonth, strategyName)', 'return fmt.Sprintf("%s | %s | %s 최적화 권고", scope, analysisMonth, strategyName)'),
        ('description.WriteString("## 执行摘要\n以下为基于本月账单的默认 FinOps 分析结果。账单不包含 CPU、内存和连接数等实时监控指标，因此空闲与低利用率项属于待核查对象。")', 'description.WriteString("## 실행 요약\n이번 달 Billing을 기준으로 생성한 기본 FinOps 분석 결과입니다. Billing에는 CPU, Memory, Connection 같은 실시간 Monitoring Metric이 없으므로 유휴 및 저활용 항목은 검증 대상입니다.")'),
        ('name = "服务资源"', 'name = "Service Resource"'),
        ('description.WriteString(fmt.Sprintf("\n%d. %s：本月成本 %.2f，建议核查使用率、闲置时段与承诺折扣；预计可节省 %.2f。", index+1, name, item.Cost, recommendationSaving))', 'description.WriteString(fmt.Sprintf("\n%d. %s: 이번 달 비용 %.2f. 사용률, 유휴 시간대, 약정 할인 적용 가능성을 검토하십시오. 예상 절감액은 %.2f입니다.", index+1, name, item.Cost, recommendationSaving))'),
        ('description.WriteString("\n\n## 空闲资源\n优先核查本月仍产生费用、但无对应运行负载或业务访问的资源；确认后可停止、释放或设置定时启停。")', 'description.WriteString("\n\n## 유휴 Resource\n이번 달에도 비용이 발생하지만 대응하는 Runtime Workload 또는 업무 Access가 없는 Resource를 우선 확인하십시오. 검증 후 중지, 해제 또는 Scheduled Start/Stop을 적용할 수 있습니다.")'),
        ('description.WriteString("\n\n## 低利用率资源\n对上述高成本计算、数据库和中间件资源核对 CPU、内存、连接数和 IOPS；连续低利用率时考虑降配。")', 'description.WriteString("\n\n## 저활용 Resource\n고비용 Compute, Database, Middleware Resource의 CPU, Memory, Connection, IOPS를 확인하십시오. 지속적으로 사용률이 낮으면 Downsize를 검토합니다.")'),
        ('description.WriteString("\n\n## 计费方式优化\n对稳定运行资源评估包年包月、节省计划或预留实例，并避免重复购买资源包。")', 'description.WriteString("\n\n## Billing 방식 최적화\n안정적으로 실행되는 Resource에는 Subscription, Savings Plan 또는 Reserved Instance를 평가하고 중복 Resource Package 구매를 방지하십시오.")'),
        ('description.WriteString("\n\n## 闲置磁盘/快照/IP\n盘点未挂载云盘、长期快照、未绑定 EIP 和闲置负载均衡；确认无依赖后清理。")', 'description.WriteString("\n\n## 유휴 Disk / Snapshot / IP\n연결되지 않은 Cloud Disk, 장기 Snapshot, 미연결 EIP, 유휴 Load Balancer를 점검하고 Dependency가 없음을 확인한 뒤 정리하십시오.")'),
        ('description.WriteString(fmt.Sprintf("\n\n## 预计可节省金额\n本月纳入分析成本 %.2f；按保守 15%% 估算，预计月节省金额 %.2f。", total, saving))', 'description.WriteString(fmt.Sprintf("\n\n## 예상 절감액\n이번 달 분석 대상 비용은 %.2f이며 보수적인 15%% 기준 예상 월 절감액은 %.2f입니다.", total, saving))'),
        ('Title: "本月云费用优化建议"', 'Title: "이번 달 Cloud 비용 최적화 권고"'),
        ('"没有可用于生成 AI 建议的费用记录"', '"no cost records are available for AI recommendation generation"'),
        ('prompt := "根据以下本地已同步云账单数据，生成简洁、可执行的中文 FinOps 优化分析。不要调用云接口，也不要编造资源、金额或监控指标。请使用 Markdown 标题，覆盖：执行摘要、空闲资源核查、低利用率资源核查、计费方式优化、闲置磁盘/快照/IP、预计可节省金额。没有实时监控数据时，必须写“建议核查”。不要输出 JSON、代码块或表格；总长度不超过 900 个中文字符。数据：" + string(contextJSON)', 'prompt := "Using the following locally synchronized cloud billing data, produce a concise and actionable Korean FinOps analysis. Do not call cloud APIs or invent resources, amounts, or monitoring metrics. Use Markdown headings covering: execution summary, idle-resource review, low-utilization review, billing-model optimization, idle disks/snapshots/IPs, and estimated savings. When real-time monitoring data is unavailable, explicitly state that validation is required. Do not output JSON, code fences, or tables. Keep the report under 900 Korean characters. Data: " + string(contextJSON)'),
        ('{"role": "system", "content": "你是严谨的 FinOps 分析师。只输出简洁中文 Markdown 分析报告，不输出 JSON。"}', '{"role": "system", "content": "You are a rigorous FinOps analyst. Output only a concise Korean Markdown analysis report and no JSON."}'),
        ('"AI 未返回可展示的 FinOps 分析内容"', '"AI returned no displayable FinOps analysis"'),
        ('recommendation.Title = "本月云费用 AI 优化建议"', 'recommendation.Title = "이번 달 Cloud 비용 AI 최적화 권고"'),
        ('recommendation.Description = "## AI 分析结论\n"', 'recommendation.Description = "## AI 분석 결론\n"'),
        ('prompt := "根据以下云费用分析工具结果生成不超过 5 条可执行优化建议。工具结果只来自本地已同步账单数据库，绝不代表实时云端数据。必须覆盖：空闲资源、低利用率资源、计费方式优化、闲置磁盘/快照/IP、预计可节省金额。账单没有实时监控指标时，明确说明需要核查而不得断言资源闲置。每条 description 不超过 80 个中文字符。只返回一个完整 JSON 对象：{\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}。不要使用 Markdown 代码块、标题或任何 JSON 之外的文字。不得编造资源；saving 必须非负且不超过 currentCost。数据：" + string(contextJSON)', 'prompt := "Using the following cloud-cost analysis tool result, generate no more than five actionable Korean optimization recommendations. The tool result comes only from the locally synchronized billing database and does not represent real-time cloud state. Cover idle resources, low utilization, billing-model optimization, idle disks/snapshots/IPs, and estimated savings. When billing lacks real-time monitoring metrics, state that validation is required rather than claiming a resource is idle. Each description must be no longer than 80 Korean characters. Return exactly one complete JSON object: {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}. Do not output Markdown fences, headings, or text outside JSON. Do not invent resources; saving must be non-negative and no greater than currentCost. Data: " + string(contextJSON)'),
        ('{"role": "system", "content": "你是严谨的 FinOps 分析师，只输出符合要求的 JSON。"}', '{"role": "system", "content": "You are a rigorous FinOps analyst. Output only JSON that satisfies the requested schema."}'),
        ('repairPrompt := "将以下 FinOps 分析结果转换为一个完整 JSON 对象。只返回 JSON，不要 Markdown 或解释。格式必须是 {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}。若原内容缺少字段，使用已知账号和成本的保守值，不得编造资源。原内容：\n" + truncateRunes(response.Content, 12000)', 'repairPrompt := "Convert the following FinOps analysis result into one complete JSON object. Return JSON only, without Markdown or explanation. The schema must be {\"recommendations\":[{\"accountId\":1,\"provider\":\"...\",\"resourceId\":\"...\",\"priority\":\"P1|P2|P3\",\"title\":\"...\",\"description\":\"...\",\"currentCost\":0,\"saving\":0}]}. If fields are missing, use conservative values from the known account and cost data and do not invent resources. Original content:\n" + truncateRunes(response.Content, 12000)'),
        ('{"role": "system", "content": "你是严格的 JSON 修复器，只输出一个有效 JSON 对象。"}', '{"role": "system", "content": "You are a strict JSON repair tool. Output exactly one valid JSON object."}'),
        ('"AI 返回的建议 JSON 无法解析: %w"', '"failed to parse AI recommendation JSON: %w"'),
        ('"AI 未生成有效的优化建议"', '"AI generated no valid optimization recommendations"'),
        ('description.WriteString("## 执行摘要\n以下为 AI 基于本月账单生成的综合优化建议：")', 'description.WriteString("## 실행 요약\n다음은 AI가 이번 달 Billing을 기준으로 생성한 종합 최적화 권고입니다.")'),
        ('description.WriteString(fmt.Sprintf("\n%d. %s：%s（当前成本 %.2f，预计节省 %.2f）", index+1, item.Title, item.Description, item.CurrentCost, item.Saving))', 'description.WriteString(fmt.Sprintf("\n%d. %s: %s (현재 비용 %.2f, 예상 절감액 %.2f)", index+1, item.Title, item.Description, item.CurrentCost, item.Saving))'),
        ('description.WriteString("\n\n## 空闲资源\n请优先验证停止但仍计费、无业务访问或无监控负载的资源。")', 'description.WriteString("\n\n## 유휴 Resource\n중지 상태지만 비용이 발생하거나 업무 Access 또는 Monitoring Load가 없는 Resource를 우선 검증하십시오.")'),
        ('description.WriteString("\n\n## 低利用率资源\n结合 CPU、内存、IOPS、连接数等监控数据确认是否应降配。")', 'description.WriteString("\n\n## 저활용 Resource\nCPU, Memory, IOPS, Connection 등의 Monitoring 데이터를 함께 확인해 Downsize 여부를 결정하십시오.")'),
        ('description.WriteString("\n\n## 计费方式优化\n评估稳定工作负载是否适合包年包月、节省计划或预留实例。")', 'description.WriteString("\n\n## Billing 방식 최적화\n안정적인 Workload에 Subscription, Savings Plan 또는 Reserved Instance가 적합한지 평가하십시오.")'),
        ('description.WriteString("\n\n## 闲置磁盘/快照/IP\n清点未挂载磁盘、长期快照、未绑定 IP 和无后端服务的负载均衡。")', 'description.WriteString("\n\n## 유휴 Disk / Snapshot / IP\n연결되지 않은 Disk, 장기 Snapshot, 미연결 IP, Backend가 없는 Load Balancer를 점검하십시오.")'),
        ('description.WriteString(fmt.Sprintf("\n\n## 预计可节省金额\n本报告覆盖成本 %.2f，AI 估算可节省金额 %.2f。", total, saving))', 'description.WriteString(fmt.Sprintf("\n\n## 예상 절감액\n이 Report의 분석 대상 비용은 %.2f이며 AI 예상 절감액은 %.2f입니다.", total, saving))'),
        ('Title: "本月云费用 AI 优化建议"', 'Title: "이번 달 Cloud 비용 AI 최적화 권고"'),
        ('"AI 未按约定返回完整 JSON，可能包含说明文字或因输出过长被截断"', '"AI did not return complete JSON; the response may contain explanatory text or may have been truncated"'),
        ('"无效的建议状态"', '"invalid recommendation status"'),
        ('"建议 ID 不能为空"', '"recommendation ID is required"'),
        ('map[string]string{"manual": "手动同步", "scheduled": "定时同步", "api": "接口触发"}', 'map[string]string{"manual": "수동 동기화", "scheduled": "Scheduled 동기화", "api": "API Trigger"}'),
        ('"支持 YYYY-MM-DD 或 RFC3339"', '"supported formats are YYYY-MM-DD or RFC3339"'),
    ],
    "backend/service/notify.go": [
        ('"通知媒介正在被规则「%s」使用，请先调整规则"', '"notification channel is used by rule %q; update the rule first"'),
        ('"所选消息模板不存在"', '"selected message template does not exist"'),
        ('"消息模板适用于%s，不能用于%s通知规则"', '"message template is scoped to %s and cannot be used by a %s notification rule"'),
        ('"部分通知媒介不存在，请重新选择"', '"some selected notification channels do not exist; select them again"'),
        ('"消息模板类型为 %s，不能发送到媒介「%s」(%s)"', '"message template type %s cannot be sent to channel %q (%s)"'),
        ('TargetName: "通知规则测试"', 'TargetName: "Notification Rule Test"'),
        ('Summary:    "这是一条由 Ops Admin 发起的通知规则测试消息"', 'Summary:    "Ops Admin에서 발송한 Notification Rule 테스트 메시지입니다."'),
        ('Detail:     "如果你收到此消息，说明模板、通知媒介和持久化投递链路工作正常。"', 'Detail:     "이 메시지를 받았다면 Template, Notification Channel, 영속 Delivery Pipeline이 정상적으로 동작합니다."'),
        ('map[string]string{"operator": "系统管理员"}', 'map[string]string{"operator": "System Administrator"}'),
        ('"通知规则不能为空"', '"notification rule is required"'),
        ('"读取消息模板失败: %w"', '"failed to read message template: %w"'),
        ('"消息模板已禁用"', '"message template is disabled"'),
        ('"消息模板适用于%s，不能处理%s事件"', '"message template is scoped to %s and cannot process a %s event"'),
        ('"通知规则未配置通知媒介"', '"notification rule has no configured channel"'),
        ('"通知规则关联的媒介不存在"', '"a channel referenced by the notification rule does not exist"'),
        ('"通知媒介已禁用"', '"notification channel is disabled"'),
        ('"消息模板与通知媒介类型不兼容"', '"message template and notification channel types are incompatible"'),
        ('return "监控告警"', 'return "Monitoring Alert"'),
        ('return "定时任务"', 'return "Scheduled Task"'),
        ('return "作业编排"', 'return "Job Orchestration"'),
        ('return "CI/CD 流水线"', 'return "CI/CD Pipeline"'),
        ('return "全部场景"', 'return "All Scopes"'),
        ('"服务重启后恢复未完成投递"', '"unfinished delivery resumed after service restart"'),
        ('"通知媒介不存在: %w"', '"notification channel does not exist: %w"'),
        ('return "触发中"', 'return "발생"'),
        ('return "已恢复"', 'return "복구"'),
        ('return "已认领"', 'return "인계됨"'),
        ('return "通知"', 'return "알림"'),
        ('return "成功"', 'return "성공"'),
        ('return "失败"', 'return "실패"'),
        ('return "执行中"', 'return "실행 중"'),
        ('return "等待人工确认"', 'return "수동 확인 대기"'),
        ('return "已拒绝"', 'return "거부됨"'),
        ('return "【定时任务】{{taskName}} · {{status}}", "**执行状态：** {{status}}\n\n**任务名称：** {{taskName}}\n**任务类型：** {{taskType}}\n**触发方式：** {{triggerType}}\n**Cron：** {{cronExpr}}\n**执行耗时：** {{duration}}\n**完成时间：** {{finishedAt}}\n\n---\n\n**执行摘要**\n{{summary}}\n\n{{detail}}"', 'return "[Scheduled Task] {{taskName}} · {{status}}", "**실행 상태:** {{status}}\n\n**Task 이름:** {{taskName}}\n**Task Type:** {{taskType}}\n**Trigger 방식:** {{triggerType}}\n**Cron:** {{cronExpr}}\n**실행 시간:** {{duration}}\n**완료 시각:** {{finishedAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}"'),
        ('return "【流水线通知】{{pipelineName}} · {{stageName}}", "**执行状态：** {{status}}\n\n**流水线：** {{pipelineName}}\n**执行编号：** #{{pipelineRunId}}\n**应用：** {{appName}}\n**环境：** {{env}}\n**分支：** {{branch}}\n**镜像版本：** {{imageTag}}\n**通知时间：** {{notifyAt}}\n\n---\n\n**执行摘要**\n{{summary}}\n\n{{detail}}"', 'return "[Pipeline Notification] {{pipelineName}} · {{stageName}}", "**실행 상태:** {{status}}\n\n**Pipeline:** {{pipelineName}}\n**Run ID:** #{{pipelineRunId}}\n**Application:** {{appName}}\n**Environment:** {{env}}\n**Branch:** {{branch}}\n**Image Version:** {{imageTag}}\n**알림 시각:** {{notifyAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}"'),
        ('return "【作业通知】{{jobName}} · {{stepName}}", "**通知类型：** {{status}}\n\n**作业名称：** {{jobName}}\n**执行编号：** #{{jobHistoryId}}\n**当前步骤：** {{stepName}}\n**触发方式：** {{triggerType}}\n**通知时间：** {{notifyAt}}\n\n---\n\n**通知摘要**\n{{summary}}\n\n{{detail}}"', 'return "[Job Notification] {{jobName}} · {{stepName}}", "**알림 Type:** {{status}}\n\n**Job 이름:** {{jobName}}\n**Run ID:** #{{jobHistoryId}}\n**현재 Step:** {{stepName}}\n**Trigger 방식:** {{triggerType}}\n**알림 시각:** {{notifyAt}}\n\n---\n\n**알림 요약**\n{{summary}}\n\n{{detail}}"'),
        ('"webhook 地址为空"', '"webhook URL is empty"'),
        ('"Webhook 返回 HTTP %d"', '"webhook returned HTTP %d"'),
        ('"平台返回业务错误码 %s: %s"', '"platform returned business error code %s: %s"'),
        ('firstNonEmpty(message, "未知错误")', 'firstNonEmpty(message, "unknown error")'),
        ('"仅失败的投递记录可以重新发送"', '"only failed delivery records can be resent"'),
    ],
    "backend/service/database_backup.go": [
        ('s.RunDatabaseBackup(plan.ID, "schedule", "定时任务")', 's.RunDatabaseBackup(plan.ID, "schedule", "Scheduled Task")'),
        ('"请填写计划名称并选择数据库"', '"enter a plan name and select a database"'),
        ('"Cron 表达式格式不正确"', '"invalid Cron expression format"'),
        ('"请选择备份计划"', '"select a backup plan"'),
        ('"请选择需要备份的业务库"', '"select a business database to back up"'),
        ('"请选择需要备份的数据库"', '"select a database to back up"'),
        ('"备份完成（"', '"Backup completed ("'),
        ('+ "）"', '+ ")"'),
        ('fmt.Sprintf("内置 PostgreSQL 逻辑备份，%d 行数据", rowCount)', 'fmt.Sprintf("built-in PostgreSQL logical backup, %d rows", rowCount)'),
        ('fmt.Sprintf("内置 MySQL 逻辑备份，%d 行数据", rowCount)', 'fmt.Sprintf("built-in MySQL logical backup, %d rows", rowCount)'),
        ('"数据库连接未设置默认库，请选择需要备份的业务库"', '"database connection has no default database; select a business database to back up"'),
        ('"不允许备份 PostgreSQL 系统 Schema，请选择业务 Schema"', '"PostgreSQL system schemas cannot be backed up; select a business schema"'),
        ('"不允许备份 MySQL 系统库，请选择业务库"', '"MySQL system databases cannot be backed up; select a business database"'),
        ('"%s 暂不支持逻辑备份，目前支持 MySQL 和 PostgreSQL"', '"%s does not support logical backup; supported engines are MySQL and PostgreSQL"'),
        ('"创建一致性备份快照失败: %w"', '"failed to create a consistent backup snapshot: %w"'),
        ('"读取表 %s 的建表语句失败: %w"', '"failed to read the CREATE TABLE statement for %s: %w"'),
        ('"读取视图 %s 的定义失败: %w"', '"failed to read view definition for %s: %w"'),
        ('"提交备份快照失败: %w"', '"failed to commit backup snapshot: %w"'),
        ('"读取表 %s 的字段定义失败: %w"', '"failed to read column definitions for table %s: %w"'),
        ('"表 %s 没有可备份的字段"', '"table %s has no columns available for backup"'),
        ('"读取表 %s 数据失败: %w"', '"failed to read data from table %s: %w"'),
        ('"未返回建库或建表语句"', '"no CREATE DATABASE or CREATE TABLE statement was returned"'),
        ('"未找到 %s 字段"', '"column %s was not found"'),
        ('"读取触发器列表失败: %w"', '"failed to read trigger list: %w"'),
        ('"读取触发器 %s 失败: %w"', '"failed to read trigger %s: %w"'),
        ('"读取存储程序列表失败: %w"', '"failed to read stored-routine list: %w"'),
        ('"读取%s %s 失败: %w"', '"failed to read %s %s: %w"'),
        ('"读取事件列表失败: %w"', '"failed to read event list: %w"'),
        ('"读取事件 %s 失败: %w"', '"failed to read event %s: %w"'),
        ('"备份文件尚不可下载"', '"backup file is not available for download"'),
        ('"请选择目标数据库连接和 Schema"', '"select a target database connection and schema"'),
        ('"恢复备份前必须完成风险确认"', '"backup restoration requires risk confirmation"'),
        ('"选择的备份记录不存在"', '"selected backup record does not exist"'),
        ('"选择的备份尚未成功或备份内容为空"', '"selected backup did not succeed or contains no backup data"'),
        ('"备份源数据库与目标数据库类型不一致，不允许跨引擎恢复"', '"source and target database types differ; cross-engine restoration is not allowed"'),
        ('"请选择平台备份或上传 SQL 备份文件"', '"select a platform backup or upload an SQL backup file"'),
        ('"备份文件不能超过 50MB"', '"backup file must not exceed 50 MB"'),
        ('"备份导入仅支持 .sql 文件"', '"backup import supports only .sql files"'),
        ('"等待恢复备份"', '"Pending backup restoration"'),
        ('"正在解析备份文件"', '"Parsing backup file"'),
        ('"备份文件中没有可执行的 SQL"', '"backup file contains no executable SQL"'),
        ('"第 %d/%d 条 SQL 恢复失败: %w"', '"SQL restore statement %d/%d failed: %w"'),
        ('"正在恢复 %d/%d"', '"Restoring %d/%d"'),
        ('"恢复备份: "+task.FileName', '"Restore Backup: "+task.FileName'),
        ('"备份恢复完成，影响 %d 行，耗时 %s"', '"backup restoration completed; %d rows affected in %s"'),
    ],
}

HAN = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF]")


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    changed_files = 0
    replacement_count = 0
    failures: list[str] = []
    for relative_path, replacements in REPLACEMENTS.items():
        path = root / relative_path
        original = path.read_text(encoding="utf-8")
        updated = original
        for old, new in replacements:
            count = updated.count(old)
            if count:
                updated = updated.replace(old, new)
                replacement_count += count
        if updated != original:
            path.write_text(updated, encoding="utf-8")
            changed_files += 1
        remaining = [(n, line) for n, line in enumerate(updated.splitlines(), 1) if HAN.search(line)]
        if remaining:
            preview = "; ".join(f"{n}: {line.strip()}" for n, line in remaining[:12])
            failures.append(f"{relative_path} still contains Han characters: {preview}")
    print(f"batch6 localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
