#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
PATH = ROOT / "backend/service/integration_ai.go"
HAN = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF]")

SIMPLE = [
    ('"环境编码，例如 dev、test、prod；留空查询全部环境"', '"Environment Code(예: dev, test, prod). 비워두면 전체 Environment를 조회합니다."'),
    ('"最多返回多少条，范围 1 到 50，默认 20"', '"최대 반환 건수. 범위 1~50, 기본값 20"'),
    ('"云账号 ID；留空则分析全部已同步云账号"', '"Cloud Account ID. 비워두면 동기화된 모든 Cloud Account를 분석합니다."'),
    ('"分析账期，格式 YYYY-MM；留空默认最近一个有本地同步账单的自然月"', '"분석 Billing Month(YYYY-MM). 비워두면 Local Billing이 있는 최근 Calendar Month를 사용합니다."'),
    ('"趋势月份数，1 到 12，默认 6"', '"Trend Month 수. 범위 1~12, 기본값 6"'),
    ('"云产品名称关键词；例如 负载均衡、NAT网关、ECS"', '"Cloud Product Keyword(예: Load Balancer, NAT Gateway, ECS)"'),
    ('"是否返回该云产品按实例/资源 ID 聚合的费用；询问实例数量或每实例费用时设为 true"', '"Cloud Product 비용을 Instance/Resource ID 기준으로 집계할지 여부. Instance 수 또는 Instance별 비용 질의에는 true를 사용합니다."'),
    ('"需要检索的关键词或问题，例如：发布流程、Redis 故障处理"', '"검색할 Keyword 또는 Question(예: Release Process, Redis 장애 대응)"'),
    ('"可选，限定检索某篇知识库文档的 ID"', '"선택 사항. 특정 Knowledge Base 문서 ID로 검색 범위를 제한합니다."'),
    ('"最多返回多少个匹配片段，范围 1 到 10，默认 5"', '"최대 Match Fragment 수. 범위 1~10, 기본값 5"'),
    ('"日志数据源 ID；留空则查询所有启用的 Elasticsearch 和 VictoriaLogs 数据源"', '"Log Datasource ID. 비워두면 활성화된 모든 Elasticsearch와 VictoriaLogs Datasource를 조회합니다."'),
    ('"数据源名称关键词，仅在未指定 datasourceId 时使用"', '"Datasource 이름 Keyword. datasourceId를 지정하지 않았을 때만 사용합니다."'),
    ('"Elasticsearch 索引或索引模式，例如 logs-*；留空查询全部索引"', '"Elasticsearch Index 또는 Pattern(예: logs-*). 비워두면 전체 Index를 조회합니다."'),
    ('"Stream/Topic，多个值用逗号分隔；默认按包含关系匹配，例如 err.log 可匹配 app.err.log.1053；使用 =app.err.log.1053 可精确匹配"', '"Stream/Topic. 여러 값은 comma로 구분합니다. 기본값은 contains match이며 =app.err.log.1053 형식은 exact match입니다."'),
    ('"日志条件：Elasticsearch 使用 Lucene，VictoriaLogs 使用 LogsQL；例如 level:ERROR，留空表示全部"', '"Log 조건. Elasticsearch는 Lucene, VictoriaLogs는 LogsQL을 사용합니다. 예: level:ERROR. 비워두면 전체입니다."'),
    ('"开始时间，支持 RFC3339、YYYY-MM-DD HH:mm:ss、昨天 10:00 或 yesterday 10:00，时区 Asia/Shanghai"', '"시작 시각. RFC3339, YYYY-MM-DD HH:mm:ss, 어제 10:00 또는 yesterday 10:00을 지원하며 Timezone은 Asia/Seoul입니다."'),
    ('"结束时间，格式与 startTime 相同，必须晚于开始时间"', '"종료 시각. startTime과 같은 형식이며 시작 시각보다 늦어야 합니다."'),
    ('"返回模式：count 仅统计数量，list 返回少量日志明细；默认 count"', '"반환 Mode. count는 건수만, list는 일부 Log Detail을 반환합니다. 기본값은 count입니다."'),
    ('"list 模式最多返回的日志条数，范围 1 到 50，默认 20"', '"list Mode의 최대 Log 수. 범위 1~50, 기본값 20"'),
    ('"数据源名称关键词"', '"Datasource 이름 Keyword"'),
    ('"数据源类型：prometheus、victoriametrics、elasticsearch 或 victorialogs"', '"Datasource Type: prometheus, victoriametrics, elasticsearch 또는 victorialogs"'),
    ('"健康状态，例如 healthy、unhealthy、unknown"', '"상태(예: healthy, unhealthy, unknown)"'),
    ('"返回条数，1 到 50，默认 20"', '"반환 건수. 범위 1~50, 기본값 20"'),
    ('"规则名、指标、摘要或标签关键词"', '"Rule 이름, Metric, Summary 또는 Label Keyword"'),
    ('"告警状态，例如 firing、claimed、recovered、resolved"', '"Alert 상태(예: firing, claimed, recovered, resolved)"'),
    ('"告警等级：P0、P1、P2、P3"', '"Alert Severity: P0, P1, P2, P3"'),
    ('"可选，RFC3339 或 YYYY-MM-DD HH:mm:ss"', '"선택 사항. RFC3339 또는 YYYY-MM-DD HH:mm:ss"'),
    ('"CMDB 主机 ID；与 keyword 二选一"', '"CMDB Host ID. keyword와 둘 중 하나를 사용합니다."'),
    ('"主机名、别名或 IP"', '"Host 이름, Alias 또는 IP"'),
    ('"指标时间范围：1h、6h、24h、7d，默认 24h"', '"Metric Time Range: 1h, 6h, 24h, 7d. 기본값 24h"'),
    ('"可选，告警事件 ID"', '"선택 사항. Alert Event ID"'),
    ('"可选，主机名、别名或 IP"', '"선택 사항. Host 이름, Alias 또는 IP"'),
    ('"可选，问题、规则或指标关键词"', '"선택 사항. Issue, Rule 또는 Metric Keyword"'),
    ('"主机指标时间范围：1h、6h、24h、7d，默认 24h"', '"Host Metric Time Range: 1h, 6h, 24h, 7d. 기본값 24h"'),
    ('"监控大屏 ID"', '"Monitoring Dashboard ID"'),
    ('"大屏名称关键词；未填写 ID 时使用"', '"Dashboard 이름 Keyword. ID가 없을 때 사용합니다."'),
    ('"规则名称"', '"Rule 이름"'),
    ('"Prometheus/VictoriaMetrics 数据源 ID"', '"Prometheus/VictoriaMetrics Datasource ID"'),
    ('"已校验的 PromQL"', '"검증된 PromQL"'),
    ('"比较符：>、>=、<、<=、==、!="', '"Comparator: >, >=, <, <=, ==, !="'),
    ('"阈值数字"', '"Threshold 숫자"'),
    ('"持续秒数，默认 300"', '"지속 시간(초). 기본값 300"'),
    ('"等级：P0、P1、P2、P3，默认 P2"', '"Severity: P0, P1, P2, P3. 기본값 P2"'),
    ('"规则说明"', '"Rule 설명"'),
    ('"环境，例如 prod"', '"Environment(예: prod)"'),
    ('"K8s 集群 ID"', '"Kubernetes Cluster ID"'),
    ('"命名空间"', '"Namespace"'),
    ('"工作负载类型，例如 deployment"', '"Workload Type(예: deployment)"'),
    ('"工作负载名称"', '"Workload 이름"'),
    ('"目标副本数"', '"Target Replica 수"'),
    ('"知识库文档名称不能为空"', '"knowledge-base document name is required"'),
    ('"Markdown 内容不能为空"', '"Markdown content is required"'),
    ('"Markdown 内容不能超过 50 万字符"', '"Markdown content must not exceed 500000 characters"'),
    ('"知识库文档不存在"', '"knowledge-base document does not exist"'),
    ('"知识库文档 ID 不能为空"', '"knowledge-base document ID is required"'),
    ('"模型名称、API 地址和模型标识不能为空"', '"model name, API URL, and model identifier are required"'),
    ('"请输入有效的 OpenAI 兼容 API 地址"', '"enter a valid OpenAI-compatible API URL"'),
    ('"模型 ID 不能为空"', '"model ID is required"'),
    ('"该模型已被会话使用，请先停用模型，不能直接删除"', '"model is used by conversations and cannot be deleted directly; disable it first"'),
    ('"只回复 OK"', '"Reply only with OK"'),
    ('title = "新会话"', 'title = "새 대화"'),
    ('"请输入对话内容"', '"conversation content is required"'),
    ('"请选择一个已启用的 AI 模型"', '"select an enabled AI model"'),
    ('"不要输出内部工具调用标记或 XML/DSML。只能调用已提供的原生工具；如果没有可调用工具，请直接用简洁中文回答。"', '"Internal tool-call markers and XML/DSML must not be exposed. Use only provided native tools; when no tool is available, answer concisely in Korean."'),
    ('"该操作需要用户确认后执行"', '"사용자 확인 후 실행할 수 있습니다."'),
    ('"工具调用轮次已达到安全上限，未继续执行。请缩小查询范围后重试。"', '"Tool 호출이 안전 제한에 도달해 실행을 중단했습니다. Query 범위를 줄여 다시 시도하십시오."'),
    ('"操作已生成，请在下方确认后执行。"', '"작업을 생성했습니다. 아래에서 확인한 뒤 실행하십시오."'),
    ('"模型返回了不受支持的内部工具调用格式，未执行任何操作。请重新提问，或切换支持原生工具调用的模型。"', '"Model이 지원되지 않는 내부 Tool 호출 형식을 반환하여 아무 작업도 실행하지 않았습니다. 질문을 다시 작성하거나 Native Tool Calling을 지원하는 Model로 전환하십시오."'),
    ('conversation.Title == "新会话"', 'conversation.Title == "새 대화"'),
    ('"该历史消息包含模型未支持的内部工具调用格式，未执行任何操作。请重新发起查询。"', '"이 History Message에는 Model이 지원하지 않는 내부 Tool 호출 형식이 포함되어 있어 아무 작업도 실행하지 않았습니다. Query를 다시 실행하십시오."'),
    ('"模型请求失败: %w"', '"model request failed: %w"'),
    ('"模型 API 返回 %d: %s"', '"model API returned %d: %s"'),
    ('"模型 API 未返回有效内容"', '"model API returned no valid content"'),
    ('"未知的 AI 工具"', '"unknown AI tool"'),
    ('"PromQL 不能为空"', '"PromQL is required"'),
    ('"没有可用的 Prometheus/VictoriaMetrics 数据源"', '"no Prometheus or VictoriaMetrics datasource is available"'),
    ('"该工具只能通过待确认动作执行"', '"this tool can run only through a pending confirmation action"'),
    ('"请提供 hostId 或主机关键词"', '"provide hostId or a host keyword"'),
    ('"知识库检索关键词不能为空"', '"knowledge-base search keyword is required"'),
    ('"limit 必须在 1 到 10 之间"', '"limit must be between 1 and 10"'),
    ('"month 参数格式无效，格式为 YYYY-MM"', '"invalid month parameter; expected YYYY-MM"'),
    ('"trendMonths 必须在 1 到 12 之间"', '"trendMonths must be between 1 and 12"'),
    ('"仅使用本地已同步云账单数据；未调用云厂商接口"', '"로컬에 동기화된 Cloud Billing만 사용했으며 Cloud Provider API는 호출하지 않았습니다."'),
    ('"按本地已同步账单的 resourceId/resourceName 聚合；不会查询云厂商接口"', '"Local Billing의 resourceId/resourceName 기준으로 집계했으며 Cloud Provider API는 조회하지 않습니다."'),
    ('"开始时间无效: %w"', '"invalid start time: %w"'),
    ('"结束时间无效: %w"', '"invalid end time: %w"'),
    ('"结束时间必须晚于开始时间"', '"end time must be later than start time"'),
    ('"单次日志查询时间范围不能超过 31 天"', '"a single log query cannot span more than 31 days"'),
    ('"返回模式仅支持 count 或 list"', '"return mode supports only count or list"'),
    ('"没有匹配且已启用的 Elasticsearch/VictoriaLogs 数据源"', '"no matching enabled Elasticsearch or VictoriaLogs datasource was found"'),
    ('"所有日志数据源查询均失败: %s"', '"all log datasource queries failed: %s"'),
    ('"时间不能为空"', '"time is required"'),
    ('{prefix: "昨天", days: -1}', '{prefix: "어제", days: -1}'),
    ('{prefix: "今天", days: 0}', '{prefix: "오늘", days: 0}'),
    ('"相对时间应为‘昨天 10:00’或‘today 10:00’"', '"relative time must use the form ‘어제 10:00’ or ‘today 10:00’"'),
    ('"支持 RFC3339、YYYY-MM-DD HH:mm:ss 或‘昨天 10:00’"', '"supported formats are RFC3339, YYYY-MM-DD HH:mm:ss, or ‘어제 10:00’"'),
    ('"该动作已处理"', '"action has already been processed"'),
    ('"不支持的待确认动作"', '"unsupported pending confirmation action"'),
    ('"规则名称、数据源和 PromQL 不能为空"', '"rule name, datasource, and PromQL are required"'),
    ('"告警规则草稿已保存为停用状态，未启用通知；请在告警规则页面审核后再启用。"', '"Alert Rule Draft를 비활성 상태로 저장했으며 Notification은 활성화하지 않았습니다. Alert Rule Page에서 검토한 뒤 활성화하십시오."'),
]


def transform_tool_line(line: str) -> str:
    indent = line[: len(line) - len(line.lstrip())]
    tools = {
        'knowledge_base_search': '{Key: "knowledge_base_search", Name: "Knowledge Base 검색", Category: "Knowledge Base", Description: "Knowledge Base 관리에서 활성화된 Local Markdown 문서만 검색합니다. Internal Standard, Runbook, Technical Document 답변에 사용하며 외부 File 또는 Cloud Service에는 접근하지 않습니다.", Permission: "read", Parameters: knowledgeBaseToolSchema()},',
        'prometheus_query': '{Key: "prometheus_query", Name: "PromQL Instant Query", Category: "Monitoring Center", Description: "Prometheus 또는 VictoriaMetrics Datasource에서 Instant PromQL Query를 실행합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"datasourceId": integerProperty("Datasource ID. 비워두면 Default Datasource를 사용합니다."), "query": stringProperty("PromQL Query")}, []string{"query"})},',
        'monitor_log_query': '{Key: "monitor_log_query", Name: "Log Instant Query", Category: "Monitoring Center", Description: "Elasticsearch 또는 VictoriaLogs에서 Time Range 기준으로 Log를 조회하고 Match Count와 일부 Detail을 반환합니다.", Permission: "read", Parameters: logQueryToolSchema()},',
        'monitor_dashboard_list': '{Key: "monitor_dashboard_list", Name: "Monitoring Dashboard 조회", Category: "Grafana Visualization", Description: "Platform Monitoring Dashboard와 Panel Overview를 조회해 Visualization 기반 Troubleshooting Entry를 제공합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"keyword": stringProperty("Dashboard 이름 Keyword")}, nil)},',
        'monitor_datasource_query': '{Key: "monitor_datasource_query", Name: "Monitoring Datasource 조회", Category: "Monitoring Skill", Description: "연결된 Monitoring 및 Log Datasource의 Type, Health, Latency, Last Check를 조회하며 Credential은 반환하지 않습니다.", Permission: "read", Parameters: datasourceQueryToolSchema()},',
        'monitor_alert_event_query': '{Key: "monitor_alert_event_query", Name: "Alert Event 조회", Category: "Monitoring Skill", Description: "Keyword, Status, Severity, Time Range 기준으로 Alert Event를 조회하고 Count와 제한된 Detail을 반환합니다.", Permission: "read", Parameters: alertEventQueryToolSchema()},',
        'host_health_diagnose': '{Key: "host_health_diagnose", Name: "Host 상태 진단", Category: "Monitoring Skill", Description: "CMDB Host 정보, 최근 24시간 CPU/Memory/Disk Metric, 연관 Alert를 결합해 상태 Evidence를 반환합니다. 수정 작업은 실행하지 않습니다.", Permission: "read", Parameters: hostHealthToolSchema()},',
        'ops_troubleshooting': '{Key: "ops_troubleshooting", Name: "지능형 Troubleshooting", Category: "Monitoring Skill", Description: "Alert ID, Host 또는 Issue Keyword를 기준으로 Alert, Host 상태, Datasource 상태를 수집해 Evidence 기반 Troubleshooting Context를 구성합니다.", Permission: "read", Parameters: troubleshootingToolSchema()},',
        'monitor_dashboard_analyze': '{Key: "monitor_dashboard_analyze", Name: "Monitoring Dashboard 분석", Category: "Monitoring Skill", Description: "Dashboard와 Panel Definition, Datasource, PromQL, Description을 읽어 Metric 의미와 Troubleshooting Entry를 분석합니다. Dashboard는 수정하지 않습니다.", Permission: "read", Parameters: dashboardAnalyzeToolSchema()},',
        'monitor_alert_rule_draft': '{Key: "monitor_alert_rule_draft", Name: "Alert Rule Draft", Category: "Monitoring Skill", Description: "기본 비활성 및 Notification 미전송 상태의 Alert Rule Draft를 생성합니다. 사용자 확인 후 저장하며 이후 Alert Rule Page에서 검토하고 활성화해야 합니다.", Permission: "write", RequireConfirmation: true, Parameters: alertRuleDraftToolSchema()},',
        'finops_cost_analysis': '{Key: "finops_cost_analysis", Name: "Cloud 비용 분석", Category: "Cloud Cost FinOps", Description: "Billing Sync를 통해 Local Database에 저장된 Cloud Cost만 조회합니다. Overview, Trend, Product/Region Breakdown을 반환하며 Cloud Provider API를 호출하거나 Billing을 동기화하지 않습니다.", Permission: "read", Parameters: finOpsAnalysisToolSchema()},',
        'asset_host_list': '{Key: "asset_host_list", Name: "Server Asset", Category: "Asset Management", Description: "CMDB Server, IP, Environment, Host Group, Online Status를 조회하며 Login Credential은 반환하지 않습니다.", Permission: "read", Parameters: assetQuerySchema("Server 이름, Alias 또는 IP Keyword")},',
        'asset_mysql_list': '{Key: "asset_mysql_list", Name: "MySQL Asset", Category: "Asset Management", Description: "관리 중인 MySQL Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},',
        'asset_postgresql_list': '{Key: "asset_postgresql_list", Name: "PostgreSQL Asset", Category: "Asset Management", Description: "관리 중인 PostgreSQL Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},',
        'asset_redis_list': '{Key: "asset_redis_list", Name: "Redis Asset", Category: "Asset Management", Description: "관리 중인 Redis Instance, Logical DB, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Redis 이름, 주소 또는 Logical DB Keyword")},',
        'asset_mongodb_list': '{Key: "asset_mongodb_list", Name: "MongoDB Asset", Category: "Asset Management", Description: "관리 중인 MongoDB Connection, Environment, Version, Health Status를 조회합니다.", Permission: "read", Parameters: assetQuerySchema("Database 이름, 주소 또는 Default Database Keyword")},',
        'k8s_list_clusters': '{Key: "k8s_list_clusters", Name: "Kubernetes Cluster 목록", Category: "Kubernetes", Description: "연결된 Cluster의 Status, Version, Node Count를 조회합니다.", Permission: "read", Parameters: objectSchema(map[string]any{}, nil)},',
        'k8s_cluster_overview': '{Key: "k8s_cluster_overview", Name: "Kubernetes Cluster Overview", Category: "Kubernetes", Description: "지정한 Cluster의 Node, Workload, Pod, Health Overview를 조회합니다.", Permission: "read", Parameters: objectSchema(map[string]any{"clusterId": integerProperty("Kubernetes Cluster ID")}, []string{"clusterId"})},',
        'k8s_restart_workload': '{Key: "k8s_restart_workload", Name: "Kubernetes Workload Restart", Category: "Kubernetes", Description: "Deployment, StatefulSet 또는 DaemonSet에 Rolling Restart를 실행합니다.", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(false)},',
        'k8s_scale_workload': '{Key: "k8s_scale_workload", Name: "Kubernetes Workload Scale", Category: "Kubernetes", Description: "Deployment 또는 StatefulSet의 Replica 수를 변경합니다.", Permission: "write", RequireConfirmation: true, Parameters: workloadActionSchema(true)},',
    }
    for key, replacement in tools.items():
        if f'Key: "{key}"' in line:
            return indent + replacement
    stripped = line.strip()
    if stripped.startswith('base := "你是 Ops Admin'):
        return indent + 'base := "당신은 Ops Admin Platform의 DevOps/SRE Assistant입니다. 답변은 한국어로 작성하고 결론, Evidence, 권고 작업 순서로 설명하십시오. 표준 Markdown을 사용하되 내부 XML, DSML, tool_calls, invoke 또는 기타 Tool Protocol을 노출하지 마십시오. Production 변경은 Risk를 명시하고 실제로 실행하지 않은 작업을 실행했다고 주장하지 마십시오. Cloud 비용 Question에는 Local Billing Data만 사용하고 Cloud Provider API를 호출하거나 Billing Data를 실시간 Cloud 상태로 표현하지 마십시오."'
    if stripped.startswith('base += "\\n\\n夜莺监控技能规范'):
        return indent + 'base += "\\n\\nMonitoring Skill 규칙: PromQL 요청은 Expression을 먼저 생성하고 필요하면 prometheus_query로 검증합니다. Alert 조회는 monitor_alert_event_query, Datasource 조회는 monitor_datasource_query, Host Issue는 host_health_diagnose, 종합 장애는 ops_troubleshooting, Dashboard Issue는 monitor_dashboard_analyze를 우선 사용합니다. 분석은 Tool Evidence를 근거로 하고 확인된 사실과 추정을 구분합니다. Alert 생성은 monitor_alert_rule_draft만 사용하며 사용자 확인 후 비활성 Draft로 저장합니다."'
    if stripped.startswith('return base + "\\n\\n附加指令'):
        return indent + 'return base + "\\n\\n추가 지침:\\n" + strings.TrimSpace(custom)'
    if stripped.startswith('const finOpsChatResponseInstruction = "云费用工具已返回结果'):
        return indent + 'const finOpsChatResponseInstruction = "Cloud Cost Tool이 결과를 반환했습니다. 8줄 이내의 간결한 한국어 Markdown으로 답하십시오. 일반 비용 Question은 Billing Month와 Account, 총 비용, Top 3 Product, Region 요약, 최대 3개 확인 항목을 포함합니다. 실시간 Monitoring Data가 없으면 유휴 상태를 단정하지 말고 검증 필요성을 명시하십시오. Instance 수 또는 Instance별 비용 Question에는 resourceBreakdown을 우선 사용하고 최대 5개 Instance 이름/ID와 비용을 보여주십시오. Data Source가 Local Billing임을 마지막에 명시하십시오."'
    if stripped.startswith('const knowledgeBaseChatResponseInstruction = "知识库检索工具已返回'):
        return indent + 'const knowledgeBaseChatResponseInstruction = "Knowledge Base Search Tool이 Local Markdown Fragment를 반환했습니다. 문서 목차를 나열하지 말고 User Question에 맞게 재구성하십시오. 1~2문장 결론 뒤에 Platform Position, 즉시 사용 가능한 Capability, 권장 사용 경로 순서로 설명하고 구현 완료와 제안을 명확히 구분하십시오. Source 또는 Original Text를 요청한 경우에만 문서 이름이나 Quote를 표시하십시오."'
    return line


def main() -> int:
    text = PATH.read_text(encoding="utf-8")
    replacements = 0
    for old, new in SIMPLE:
        count = text.count(old)
        if count:
            text = text.replace(old, new)
            replacements += count
    lines = text.splitlines()
    transformed = [transform_tool_line(line) for line in lines]
    PATH.write_text("\n".join(transformed) + "\n", encoding="utf-8")
    remaining = [(n, line) for n, line in enumerate(transformed, 1) if HAN.search(line)]
    print(f"batch7 localized at least {replacements} direct occurrence(s); {len(remaining)} Han line(s) remain")
    for n, line in remaining[:20]:
        print(f"REMAINING: {n}: {line.strip()}", file=sys.stderr)
    return 1 if remaining else 0


if __name__ == "__main__":
    raise SystemExit(main())
