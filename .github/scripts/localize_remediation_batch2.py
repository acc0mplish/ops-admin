#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/service/platform.go": [
        ('{Name: "开发环境", Code: "dev", Sort: 10, Status: 1, Description: "开发联调与日常验证环境"}', '{Name: "Development", Code: "dev", Sort: 10, Status: 1, Description: "Development integration and routine validation environment"}'),
        ('{Name: "测试环境", Code: "test", Sort: 20, Status: 1, Description: "功能测试、集成测试与预发布验证环境"}', '{Name: "Test", Code: "test", Sort: 20, Status: 1, Description: "Functional, integration, and pre-release validation environment"}'),
        ('{Name: "生产环境", Code: "prod", Sort: 30, Status: 1, Description: "正式对外服务环境"}', '{Name: "Production", Code: "prod", Sort: 30, Status: 1, Description: "Production environment serving external traffic"}'),
        ('"环境名称和环境标识不能为空"', '"environment name and code are required"'),
        ('"环境标识创建后不可修改，请新建环境并迁移资源"', '"environment code cannot be changed after creation; create a new environment and migrate resources"'),
        ('"请选择要删除的环境"', '"select an environment to delete"'),
        ('{name: "应用", entity:', '{name: "application", entity:'),
        ('{name: "主机", entity:', '{name: "host", entity:'),
        ('{name: "数据库", entity:', '{name: "database", entity:'),
        ('{name: "K8s 集群", entity:', '{name: "Kubernetes cluster", entity:'),
        ('{name: "监控数据源", entity:', '{name: "monitoring datasource", entity:'),
        ('{name: "告警规则", entity:', '{name: "alert rule", entity:'),
        ('"环境“%s”仍被 %d 个%s引用，请先迁移相关资源"', '"environment %q is still referenced by %d %s resource(s); migrate them first"'),
        ('"请选择要触发的作业"', '"select a job to trigger"'),
        ('"联动处置触发失败"', '"linked remediation trigger failed"'),
        ('"已触发作业执行"', '"job execution triggered"'),
        ('"已记录诊断处置动作，可在快速执行或作业中继续处理"', '"diagnostic remediation action recorded; continue from Quick Execution or Jobs"'),
        ('"已触发联动处置"', '"linked remediation triggered"'),
        ('case "开发", "开发环境":', 'case "dev", "development", "\\u5f00\\u53d1", "\\u5f00\\u53d1\\u73af\\u5883":'),
        ('case "测试", "测试环境":', 'case "test", "testing", "\\u6d4b\\u8bd5", "\\u6d4b\\u8bd5\\u73af\\u5883":'),
        ('case "生产", "生产环境":', 'case "prod", "production", "\\u751f\\u4ea7", "\\u751f\\u4ea7\\u73af\\u5883":'),
    ],
    "backend/service/database_multidb.go": [
        ('"%s 暂不支持该操作，请在工作台使用该类型支持的能力"', '"%s does not support this operation; use capabilities supported by this database type in the workbench"'),
        ('"%s 暂不支持表数据导入"', '"%s does not support table-data import"'),
        ('"源表不存在或没有可导入字段"', '"source table does not exist or has no importable columns"'),
        ('"筛选字段不存在"', '"filter column does not exist"'),
        ('"当前表没有主键，不能直接修改或删除数据"', '"the current table has no primary key; rows cannot be updated or deleted directly"'),
        ('"缺少主键字段 %s"', '"missing primary-key column %s"'),
        ('"数据表不存在或没有可读取的字段"', '"table does not exist or has no readable columns"'),
        ('"没有可插入的数据"', '"no rows are available to insert"'),
        ('"当前表没有主键，不能直接编辑数据"', '"the current table has no primary key; rows cannot be edited directly"'),
        ('fmt.Sprintf("%d 秒", ttlSeconds)', 'fmt.Sprintf("%d seconds", ttlSeconds)'),
        ('"永不过期"', '"Never expires"'),
        ('"已过期"', '"Expired"'),
        ('"请输入 Redis 命令"', '"enter a Redis command"'),
        ('"Redis 命令中的引号未闭合"', '"Redis command contains an unclosed quote"'),
        ('"所选数据库不是 Redis"', '"selected database is not Redis"'),
        ('"Redis 命令控制台不允许执行 %s"', '"Redis command console does not allow %s"'),
        ('"只读命令，可直接执行"', '"read-only command; execution is allowed"'),
        ('"写入命令，需要二次确认"', '"write command; explicit confirmation is required"'),
        ('"当前 Redis 连接为只读模式"', '"current Redis connection is read-only"'),
        ('"当前 Redis 连接为只读模式，不能执行写入命令"', '"current Redis connection is read-only; write commands are not allowed"'),
        ('"写入命令需要确认后才能执行"', '"write command requires confirmation before execution"'),
    ],
    "backend/service/ops_job.go": [
        ('"作业编排定义不能为空"', '"job orchestration definition is required"'),
        ('"作业编排定义格式不正确"', '"invalid job orchestration definition format"'),
        ('"请至少添加一个作业步骤"', '"add at least one job step"'),
        ('"存在缺少 ID 的作业步骤"', '"a job step is missing its ID"'),
        ('fmt.Sprintf("步骤 %d", index+1)', 'fmt.Sprintf("Step %d", index+1)'),
        ('"步骤“%s”：%w"', '"step %q: %w"'),
        ('"作业名称不能为空"', '"job name is required"'),
        ('"作业 ID 不能为空"', '"job ID is required"'),
        ('"作业模板名称不能为空"', '"job template name is required"'),
        ('"作业模板 ID 不能为空"', '"job template ID is required"'),
        ('"当前作业已禁用，无法执行"', '"the current job is disabled and cannot be executed"'),
        ('"作业已触发，正在执行"', '"job triggered and running"'),
        ('"当前作业不处于待确认状态"', '"the current job is not awaiting approval"'),
        ('"当前步骤不处于待确认状态"', '"the current step is not awaiting approval"'),
        ('"人工确认已通过"', '"manual approval granted"'),
        ('"作业定义已损坏，无法继续执行"', '"job definition is invalid and execution cannot continue"'),
        ('"人工确认已拒绝"', '"manual approval rejected"'),
        ('"作业被人工确认步骤拒绝"', '"job was rejected at a manual approval step"'),
        ('"作业编排中存在循环依赖，请检查连线关系"', '"job orchestration contains a cyclic dependency; review step connections"'),
        ('"作业执行完成"', '"job execution completed"'),
        ('"步骤开始执行"', '"step execution started"'),
        ('"正在执行步骤：%s"', '"executing step: %s"'),
        ('"等待人工确认"', '"awaiting manual approval"'),
        ('"请确认后继续执行该作业"', '"approve to continue job execution"'),
        ('"等待人工确认：%s"', '"awaiting manual approval: %s"'),
        ('"缺少脚本配置"', '"missing script configuration"'),
        ('"作业步骤执行中"', '"job step running"'),
        ('"文件分发配置不完整"', '"file-distribution configuration is incomplete"'),
        ('"缺少通知规则"', '"missing notification rule"'),
        ('"通知已触发"', '"notification triggered"'),
    ],
    "backend/service/ops_schedule.go": [
        ('"变量 VARIABLE_%s 未在脚本中声明"', '"variable VARIABLE_%s is not declared by the script"'),
        ('"读取变量 VARIABLE_%s 失败: %w"', '"failed to read variable VARIABLE_%s: %w"'),
        ('"请配置必填变量 VARIABLE_%s"', '"configure required variable VARIABLE_%s"'),
        ('"请求头必须是 JSON 对象"', '"request headers must be a JSON object"'),
        ('"Cron 表达式不能为空"', '"Cron expression is required"'),
        ('"任务名称不能为空"', '"task name is required"'),
        ('"Cron 表达式格式不正确"', '"invalid Cron expression format"'),
        ('"请选择脚本"', '"select a script"'),
        ('"脚本已禁用，不能用于定时任务"', '"disabled script cannot be used by a scheduled task"'),
        ('"请选择目标主机或主机组"', '"select target hosts or a host group"'),
        ('"目标主机和主机组只能二选一"', '"target hosts and host groups are mutually exclusive"'),
        ('"HTTP 地址不能为空"', '"HTTP URL is required"'),
        ('"不支持的任务类型"', '"unsupported task type"'),
        ('"请选择任务"', '"select a task"'),
        ('"模板名称不能为空"', '"template name is required"'),
        ('"不支持的模板类型"', '"unsupported template type"'),
        ('"该模板仍被任务引用，不能删除"', '"template is still referenced by tasks and cannot be deleted"'),
        ('"severity":       "定时任务"', '"severity":       "Scheduled Task"'),
        ('"HTTP 探针返回 %d，符合预期状态码 %d。"', '"HTTP probe returned %d, matching expected status %d."'),
        ('"HTTP 探针返回 %d，未达到预期状态码 %d。完整响应内容请在任务日志中查看。"', '"HTTP probe returned %d instead of expected status %d. See task logs for the complete response."'),
        ('"HTTP 探针请求失败。完整错误信息请在任务日志中查看。"', '"HTTP probe request failed. See task logs for complete error details."'),
        ('"\\n...（执行详情已截断，请在任务日志中查看完整输出）"', '"\\n...(execution details truncated; see task logs for complete output)"'),
        ('return "HTTP 探针"', 'return "HTTP Probe"'),
        ('return "脚本任务"', 'return "Script Task"'),
        ('return "手动执行"', 'return "Manual"'),
        ('return "定时触发"', 'return "Scheduled"'),
        ('fmt.Sprintf("%d 毫秒", value.Milliseconds())', 'fmt.Sprintf("%d ms", value.Milliseconds())'),
        ('fmt.Sprintf("定时任务 - %s", task.Name)', 'fmt.Sprintf("Scheduled Task - %s", task.Name)'),
        ('"定时任务执行中"', '"scheduled task running"'),
        ('"执行完成"', '"execution completed"'),
        ('"%s，期望状态码 %d"', '"%s; expected HTTP status %d"'),
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
        if not path.exists():
            failures.append(f"missing file: {relative_path}")
            continue
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
        remaining = [(line_no, line) for line_no, line in enumerate(updated.splitlines(), 1) if HAN.search(line)]
        if remaining:
            preview = "; ".join(f"{line_no}: {line.strip()}" for line_no, line in remaining[:8])
            failures.append(f"{relative_path} still contains Han characters: {preview}")

    print(f"batch2 localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
