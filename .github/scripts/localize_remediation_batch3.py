#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/service/ops.go": [
        ('"最多定义 30 个执行变量"', '"at most 30 execution variables may be defined"'),
        ('"变量名 %q 无效；仅支持大写字母、数字和下划线，且必须以字母开头"', '"invalid variable name %q; use uppercase letters, digits, and underscores, starting with a letter"'),
        ('"变量 %s 重复"', '"duplicate variable %s"'),
        ('"变量 %s 的默认值或说明过长"', '"default value or description for variable %s is too long"'),
        ('"请填写执行变量 %s"', '"execution variable %s is required"'),
        ('"执行变量 %s 的值过长"', '"value for execution variable %s is too long"'),
        ('"变量 %s 未在脚本中定义"', '"variable %s is not defined by the script"'),
        ('"检测到高风险操作，请确认目标范围和命令内容后再次执行"', '"high-risk operation detected; confirm the target scope and command before execution"'),
        ('strings.Contains(environment, "生产")', 'strings.Contains(environment, "\\u751f\\u4ea7")'),
        ('"目标包含生产环境主机，请确认目标范围后再次执行"', '"targets include production hosts; confirm the target scope before execution"'),
        ('"脚本名称不能为空"', '"script name is required"'),
        ('"脚本内容不能为空"', '"script content is required"'),
        ('ChangeSummary: "创建脚本"', 'ChangeSummary: "Create Script"'),
        ('ChangeSummary: "历史版本归档"', 'ChangeSummary: "Archive Historical Version"'),
        ('ChangeSummary: "现有版本归档"', 'ChangeSummary: "Archive Current Version"'),
        ('fmt.Sprintf("回滚至 v%d", version)', 'fmt.Sprintf("Rollback to v%d", version)'),
        ('"执行命令不能为空"', '"command is required"'),
        ('opsTaskTitle(payload.Title, "命令执行")', 'opsTaskTitle(payload.Title, "Command Execution")'),
        ('"任务创建成功，正在执行中"', '"task created and running"'),
        ('"请选择脚本"', '"select a script"'),
        ('"当前脚本已禁用，无法执行"', '"the selected script is disabled and cannot be executed"'),
        ('opsTaskTitle(payload.Title, "脚本执行")', 'opsTaskTitle(payload.Title, "Script Execution")'),
        ('"目标路径不能为空"', '"target path is required"'),
        ('"目标路径属于系统目录，请确认目标范围后再次执行"', '"target path is a system directory; confirm the target scope before execution"'),
        ('"请上传待分发文件"', '"upload a file to distribute"'),
        ('"请选择源服务器"', '"select a source server"'),
        ('"源文件路径不能为空"', '"source file path is required"'),
        ('"不支持的文件来源类型"', '"unsupported file source type"'),
        ('opsTaskTitle(payload.Title, "文件分发")', 'opsTaskTitle(payload.Title, "File Distribution")'),
        ('"任务仍在执行中，暂不能重试"', '"task is still running and cannot be retried"'),
        ('"文件内容不会长期保存在平台，请复制原文件分发任务后重新上传"', '"file content is not retained by the platform; copy the original distribution task and upload the file again"'),
        ('"当前任务没有失败或超时主机"', '"the current task has no failed or timed-out hosts"'),
        ('task.Title + "（失败重试）"', 'task.Title + " (Failed Targets Retry)"'),
        ('"目标主机和主机组只能二选一"', '"target hosts and host groups are mutually exclusive"'),
        ('"请至少选择一台主机或一个主机组"', '"select at least one host or host group"'),
        ('fmt.Sprintf("成功 %d 台，失败 %d 台", successCount, failedCount)', 'fmt.Sprintf("%d succeeded, %d failed", successCount, failedCount)'),
        ('fmt.Sprintf("已完成 %d 台，成功 %d 台，失败 %d 台，执行中 %d 台", completedCount, successCount, failedCount, runningCount)', 'fmt.Sprintf("%d completed, %d succeeded, %d failed, %d running", completedCount, successCount, failedCount, runningCount)'),
        ('fmt.Sprintf("执行完成，成功 %d 台，失败 %d 台", successCount, failedCount)', 'fmt.Sprintf("execution completed: %d succeeded, %d failed", successCount, failedCount)'),
        ('fmt.Errorf("执行超时，已在 %d 秒后终止", timeoutSeconds)', 'fmt.Errorf("execution timed out and was terminated after %d seconds", timeoutSeconds)'),
        ('strings.Contains(err.Error(), "执行超时")', 'strings.Contains(err.Error(), "execution timed out")'),
    ],
    "backend/service/service.go": [
        ('"用户名或密码错误"', '"invalid username or password"'),
        ('"账号已停用"', '"account is disabled"'),
        ('"登录成功"', '"login successful"'),
        ('"刷新凭证缺失"', '"refresh token is missing"'),
        ('"登录会话无效"', '"login session is invalid"'),
        ('"登录已过期"', '"login session has expired"'),
        ('"账号不可用"', '"account is unavailable"'),
        ('SiteSlogan:         "个人运维管理平台"', 'SiteSlogan:         "Personal Operations Management Platform"'),
        ('LoginSubtitle:      "系统管理与运维控制台"', 'LoginSubtitle:      "System Administration and Operations Console"'),
        ('"用户名已存在"', '"username already exists"'),
        ('"默认管理员不允许删除"', '"default administrator cannot be deleted"'),
        ('"原密码不正确"', '"current password is incorrect"'),
        ('"请选择所属环境"', '"select an environment"'),
        ('"新增主机资产"', '"Create Host Asset"'),
        ('"更新主机基础信息"', '"Update Host Details"'),
        ('"同步主机配置与连接状态"', '"Synchronize Host Configuration and Connection Status"'),
        ('firstNonEmpty(Trimmed(row.Provider), "自建")', 'firstNonEmpty(Trimmed(row.Provider), "On-premises")'),
        ('"请选择认证凭据"', '"select an authentication credential"'),
        ('"所选认证凭据不存在或已停用"', '"selected authentication credential does not exist or is disabled"'),
        ('"所选访问网关不存在或已停用"', '"selected access gateway does not exist or is disabled"'),
        ('+"：未返回公网或私网 IP"', '+": no public or private IP was returned"'),
        ('+"：主机已存在，未覆盖"', '+": host already exists and was not overwritten"'),
        ('+"：新增失败："+createErr.Error()', '+": creation failed: "+createErr.Error()'),
        ('+"：查询已有主机失败"', '+": failed to query existing host"'),
        ('"删除主机资产"', '"Delete Host Asset"'),
        ('"主机组名称不能为空"', '"host group name is required"'),
        ('"主机组不存在"', '"host group does not exist"'),
        ('"当前主机组下仍有关联主机，不能删除"', '"host group still contains hosts and cannot be deleted"'),
        ('"当前主机组下仍有子分组，不能删除"', '"host group still contains child groups and cannot be deleted"'),
        ('"上级主机组不能选择自己"', '"a host group cannot be its own parent"'),
        ('"上级主机组不存在"', '"parent host group does not exist"'),
        ('"上级主机组不能选择当前分组或其子分组"', '"parent host group cannot be the current group or one of its descendants"'),
        ('"凭据已被主机关联，不能删除"', '"credential is referenced by hosts and cannot be deleted"'),
        ('"云账号已被主机关联，不能删除"', '"cloud account is referenced by hosts and cannot be deleted"'),
        ("COALESCE(NULLIF(provider, ''), '自建')", "COALESCE(NULLIF(provider, ''), 'On-premises')"),
        ('case "key", "private_key", "密钥认证":', 'case "key", "private_key", "\\u5bc6\\u94a5\\u8ba4\\u8bc1":'),
        ('"主机未配置 SSH 地址"', '"host has no SSH address configured"'),
        ('"主机未配置 SSH 用户"', '"host has no SSH user configured"'),
        ('"主机同步超时"', '"host synchronization timed out"'),
        ('"密码凭据为空"', '"password credential is empty"'),
        ('value = value + "核"', 'value = value + " cores"'),
        ('fmt.Sprintf("%d核", item.CPU)', 'fmt.Sprintf("%d cores", item.CPU)'),
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
            preview = "; ".join(f"{line_no}: {line.strip()}" for line_no, line in remaining[:10])
            failures.append(f"{relative_path} still contains Han characters: {preview}")

    print(f"batch3 localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
