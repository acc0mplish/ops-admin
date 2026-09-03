#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/controller/k8s.go": [
        ('"请先登录"', '"authentication required"'),
    ],
    "backend/controller/monitor.go": [
        ('"startDate 格式无效，应为 YYYY-MM-DD"', '"invalid startDate format; expected YYYY-MM-DD"'),
        ('"endDate 格式无效，应为 YYYY-MM-DD"', '"invalid endDate format; expected YYYY-MM-DD"'),
        ('"开始日期必须早于结束日期"', '"start date must be earlier than end date"'),
        ('"请粘贴 Prometheus Rule YAML"', '"Prometheus rule YAML is required"'),
        ('"YAML 内容不能超过 2 MB"', '"YAML content must not exceed 2 MB"'),
        ('"无效的 Prometheus 模板导入参数"', '"invalid Prometheus template import payload"'),
    ],
    "backend/internal/domain/dnsserver/server.go": [
        ('"UDP %s 启动失败: %w"', '"failed to start UDP listener %s: %w"'),
        ('"TCP %s 启动失败: %w"', '"failed to start TCP listener %s: %w"'),
        ('"未配置上游 DNS"', '"no upstream DNS server configured"'),
        ('"所有上游 DNS 查询均失败"', '"all upstream DNS queries failed"'),
    ],
    "backend/service/asset_service_diagnosis.go": [
        ('"Java、Arthas Jar 或目标进程不可用："', '"Java, the Arthas JAR, or the target process is unavailable: "'),
        ('"Java、Arthas Jar 与目标进程均已就绪，可直接执行 Arthas CLI 诊断。"', '"Java, the Arthas JAR, and the target process are ready for Arthas CLI diagnostics."'),
    ],
    "backend/service/finops_cloud_billing.go": [
        ('"阿里云官方账单 API"', '"Aliyun Official Billing API"'),
        ('"腾讯云官方账单 API"', '"Tencent Cloud Official Billing API"'),
        ('"自定义账单接口"', '"Custom Billing API"'),
        ('"%s尚未配置账单 HTTP 地址"', '"%s has no billing HTTP endpoint configured"'),
        ('"账单接口返回 HTTP %d"', '"billing API returned HTTP %d"'),
        ('"账单接口数据格式无效: %w"', '"invalid billing API response format: %w"'),
        ('"阿里云账单同步需要 AccessKey 与 SecretKey"', '"Aliyun billing synchronization requires AccessKey and SecretKey"'),
        ('"阿里云账单接口返回 HTTP %d: %s"', '"Aliyun billing API returned HTTP %d: %s"'),
        ('"解析阿里云账单响应失败: %w"', '"failed to parse Aliyun billing response: %w"'),
        ('"阿里云账单接口声明 %d 条记录，但响应结构未解析到明细"', '"Aliyun billing API declared %d records but no detail rows could be parsed"'),
        ('"阿里云账单接口返回重复的 NextToken，已停止以避免重复计费"', '"Aliyun billing API returned a repeated NextToken; stopped to avoid duplicate charges"'),
        ('"腾讯云账单同步需要 SecretId 与 SecretKey"', '"Tencent Cloud billing synchronization requires SecretId and SecretKey"'),
        ('"腾讯云账单接口返回 HTTP %d: %s"', '"Tencent Cloud billing API returned HTTP %d: %s"'),
        ('"解析腾讯云账单响应失败: %w"', '"failed to parse Tencent Cloud billing response: %w"'),
        ('"腾讯云账单接口错误: %s"', '"Tencent Cloud billing API error: %s"'),
    ],
    "backend/service/finops_cloud_billing_test.go": [
        ('"云服务器 ECS"', '"Elastic Compute Service"'),
        ('"分析结果如下：\\n```json\\n{\\"recommendations\\":[{\\"title\\":\\"检查 NAT 网关\\"}]}\\n```\\n请确认后执行。"', '"Analysis result:\\n```json\\n{\\"recommendations\\":[{\\"title\\":\\"Review NAT gateway\\"}]}\\n```\\nConfirm before execution."'),
        ('`{"recommendations":[{"title":"检查 NAT 网关"}]}`', '`{"recommendations":[{"title":"Review NAT gateway"}]}`'),
        ('"负载均衡"', '"Load Balancer"'),
        ('"检查当前 K8s 集群健康状态。"', '"Check the current Kubernetes cluster health."'),
    ],
    "backend/service/ssl_acme.go": [
        ('"创建 ACME TXT 记录失败: %w"', '"failed to create ACME TXT record: %w"'),
        ('"清理 ACME TXT 记录失败: %w"', '"failed to clean up ACME TXT record: %w"'),
        ('"证书没有申请域名"', '"certificate has no requested domain"'),
        ('map[bool]string{true: "续签证书", false: "申请证书"}', 'map[bool]string{true: "Renew Certificate", false: "Apply Certificate"}'),
        ('"创建 ACME Client 失败: %w"', '"failed to create ACME client: %w"'),
        ('"注册 ACME 账号失败: %w"', '"failed to register ACME account: %w"'),
        ('"配置 DNS-01 失败: %w"', '"failed to configure DNS-01 challenge: %w"'),
        ('"ACME 签发失败: %w"', '"ACME issuance failed: %w"'),
        ('"解析 ACME 签发结果失败: %w"', '"failed to parse ACME issuance result: %w"'),
        ('"加密 Private Key 失败: %w"', '"failed to encrypt private key: %w"'),
        ('"ACME Challenge %s 不属于主域名 %s"', '"ACME challenge %s does not belong to main domain %s"'),
    ],
    "backend/middleware/operation_log.go": [
        ('"body: SSL Certificate 与 Private Key 敏感正文，已跳过记录"', '"body: sensitive SSL certificate and private-key content omitted"'),
        ('"body: multipart/form-data 文件上传，已跳过正文记录"', '"body: multipart/form-data upload content omitted"'),
        ('"body: 请求体较大，已跳过正文记录"', '"body: oversized request content omitted"'),
        ('"...(已截断)"', '"...(truncated)"'),
        ('"主机资产管理"', '"Host Asset Management"'),
        ('"数据库资产管理"', '"Database Asset Management"'),
        ('"网关资产管理"', '"Gateway Asset Management"'),
        ('"数据库工作台"', '"Database Workbench"'),
        ('"K8s 资源管理"', '"Kubernetes Resource Management"'),
        ('"快速执行"', '"Quick Execution"'),
        ('"作业编排"', '"Job Orchestration"'),
        ('"定时任务"', '"Scheduled Tasks"'),
        ('"应用发布"', '"Application Delivery"'),
        ('"消息通知"', '"Notifications"'),
        ('"监控中心"', '"Monitoring Center"'),
        ('"域名管理"', '"Domain Management"'),
        ('"用户管理"', '"User Management"'),
        ('"角色管理"', '"Role Management"'),
        ('"菜单管理"', '"Menu Management"'),
        ('"部门管理"', '"Department Management"'),
        ('"岗位管理"', '"Position Management"'),
        ('"登录日志管理"', '"Login Log Management"'),
        ('"操作日志管理"', '"Operation Log Management"'),
    ],
    "backend/controller/controller.go": [
        ('"登录已过期，请重新登录"', '"session expired; sign in again"'),
        ('"请选择要上传的文件"', '"select a file to upload"'),
        ('"仅支持图片文件上传"', '"only image files are supported"'),
        ('"创建上传目录失败"', '"failed to create upload directory"'),
        ('"保存文件失败"', '"failed to save uploaded file"'),
        ('[]string{"主机名称*", "SSH地址*", "SSH端口", "SSH用户*", "认证凭据*", "连接方式*", "访问网关", "所属环境*", "私网IP", "公网IP", "云厂商", "所在区域", "备注"}', '[]string{"호스트 이름*", "SSH 주소*", "SSH 포트", "SSH 사용자*", "인증 Credential*", "연결 방식*", "접속 Gateway", "소속 Environment*", "Private IP", "Public IP", "Cloud Provider", "Region", "비고"}'),
        ('"自建"', '"On-premises"'),
        ('"excel导入示例"', '"Excel Import 예시"'),
        ('"填写说明"', '"작성 안내"'),
        ('"字段说明"', '"필드 설명"'),
        ('"带 * 的列为必填；目标主机组由导入弹窗统一选择。"', '"* 표시 열은 필수입니다. Target Host Group은 Import 대화상자에서 선택합니다."'),
        ('"连接方式仅支持 direct（直连）或 gateway（通过网关）；gateway 时必须填写已启用的访问网关名称。"', '"연결 방식은 direct 또는 gateway만 지원합니다. gateway 사용 시 활성화된 접속 Gateway 이름이 필요합니다."'),
        ('"认证凭据和访问网关均按名称匹配，必须已在平台中创建且启用。"', '"인증 Credential과 접속 Gateway는 이름으로 일치시키며 플랫폼에 생성되어 활성화되어 있어야 합니다."'),
        ('"所属环境填写环境编码，例如 production、test；SSH 地址建议填写内网 IP。"', '"Environment에는 production 또는 test와 같은 Code를 입력하고 SSH 주소에는 Private IP 사용을 권장합니다."'),
        ('"Excel 模板格式已更新，请下载最新模板后导入"', '"the Excel template format has changed; download the latest template before importing"'),
        ('"请先登录"', '"authentication required"'),
    ],
    "web/src/utils/i18n.js": [
        ("'超级管理员': 'superAdmin',", "'\\u8d85\\u7ea7\\u7ba1\\u7406\\u5458': 'superAdmin',"),
        ("'总部': 'headquarters'", "'\\u603b\\u90e8': 'headquarters'"),
    ],
}

HAN = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF]")


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    changed_files = 0
    replacement_count = 0
    failed: list[str] = []

    for relative_path, replacements in REPLACEMENTS.items():
        path = root / relative_path
        if not path.exists():
            failed.append(f"missing file: {relative_path}")
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

        remaining = [(index, line) for index, line in enumerate(updated.splitlines(), 1) if HAN.search(line)]
        if remaining:
            preview = "; ".join(f"{line_no}: {line.strip()}" for line_no, line in remaining[:5])
            failed.append(f"{relative_path} still contains Han characters: {preview}")

    print(f"localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failed:
        for item in failed:
            print(f"ERROR: {item}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
