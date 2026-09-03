#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/service/domain.go": [
        ('hint := "已配置"', 'hint := "Configured"'),
        ('"账号名称不能为空"', '"account name is required"'),
        ('"仅支持阿里云 DNS 和腾讯云 DNSPod"', '"only Aliyun DNS and Tencent Cloud DNSPod are supported"'),
        ('"AccessKey/SecretId 和 SecretKey 不能为空"', '"AccessKey or SecretId and SecretKey are required"'),
        ('action := "修改公网 DNS 账号"', 'action := "Update Public DNS Account"'),
        ('action = "新增公网 DNS 账号"', 'action = "Create Public DNS Account"'),
        ('"该 DNS 账号仍关联 %d 张 SSL 证书，请先迁移或删除关联证书"', '"the DNS account is still referenced by %d SSL certificate(s); migrate or delete the certificates first"'),
        ('"删除公网 DNS 账号"', '"Delete Public DNS Account"'),
        ('"DNS 账号已停用"', '"DNS account is disabled"'),
        ('"域名或记录 ID 缺失"', '"domain or record ID is missing"'),
        ('"公网解析记录"+action', '"Public DNS Record "+action'),
        ('"error": "不支持的批量操作"', '"error": "unsupported batch operation"'),
        ('"error": "批量修改记录值要求所选记录类型一致"', '"error": "batch value updates require all selected records to have the same type"'),
        ('"error": "记录不存在或不属于当前域名"', '"error": "record does not exist or does not belong to the current domain"'),
        ('"DNS 账号或域名不能为空"', '"DNS account and domain are required"'),
        ('"该域名不存在或不属于当前 DNS 账号，请先刷新域名列表"', '"domain does not exist or is not owned by the current DNS account; refresh the domain list first"'),
        ('"记录不存在或不属于当前域名"', '"record does not exist or does not belong to the current domain"'),
        ('"不支持的记录操作"', '"unsupported record operation"'),
        ('"启用内网 DNS"', '"Enable Internal DNS"'),
        ('"保存内网 DNS 设置"', '"Save Internal DNS Settings"'),
        ('map[bool]string{true: "新增内网 Zone", false: "修改内网 Zone"}', 'map[bool]string{true: "Create Internal DNS Zone", false: "Update Internal DNS Zone"}'),
        ('"删除内网 Zone"', '"Delete Internal DNS Zone"'),
        ('"Zone 不存在"', '"DNS zone does not exist"'),
        ('"A 记录值必须是合法 IPv4 地址"', '"A record value must be a valid IPv4 address"'),
        ('"内网解析第一阶段仅支持 A 和 CNAME"', '"the current internal DNS phase supports only A and CNAME records"'),
        ('"解析记录不存在或不属于当前 Zone"', '"DNS record does not exist or does not belong to the current zone"'),
        ('map[bool]string{true: "新增内网解析记录", false: "修改内网解析记录"}', 'map[bool]string{true: "Create Internal DNS Record", false: "Update Internal DNS Record"}'),
        ('"删除内网解析记录"', '"Delete Internal DNS Record"'),
        ('map[string]string{"create": "批量新增内网解析记录", "update": "批量修改内网解析记录", "delete": "批量删除内网解析记录", "enable": "批量启用内网解析记录", "disable": "批量禁用内网解析记录"}', 'map[string]string{"create": "Batch Create Internal DNS Records", "update": "Batch Update Internal DNS Records", "delete": "Batch Delete Internal DNS Records", "enable": "Batch Enable Internal DNS Records", "disable": "Batch Disable Internal DNS Records"}'),
        ('"不支持的批量操作"', '"unsupported batch operation"'),
        ('"请至少提供一条解析记录"', '"provide at least one DNS record"'),
        ('"第 %d 条记录：%w"', '"record %d: %w"'),
        ('"第 %d 条记录缺少 ID"', '"record %d is missing an ID"'),
        ('"第 %d 条记录不存在"', '"record %d does not exist"'),
        ('"第 %d 条记录保存失败：%w"', '"failed to save record %d: %w"'),
        ('"请至少选择一条解析记录"', '"select at least one DNS record"'),
        ('"部分解析记录不存在或不属于当前 Zone"', '"some DNS records do not exist or do not belong to the current zone"'),
        ('fmt.Sprintf("%d 条", affected)', 'fmt.Sprintf("%d records", affected)'),
        ('"测试类型仅支持 A 或 CNAME"', '"test type supports only A or CNAME"'),
        ('"主机记录不能为空"', '"host record is required"'),
        ('"主机记录 %q 格式不合法"', '"invalid host record format: %q"'),
        ('"Zone 必须是完整域名，例如 ops.internal"', '"zone must be a fully qualified domain name, for example ops.internal"'),
        ('"CNAME 记录值必须是完整域名"', '"CNAME record value must be a fully qualified domain name"'),
        ('"同一名称不能同时配置 CNAME 和 A 记录"', '"CNAME and A records cannot coexist for the same name"'),
        ('"CNAME 不能指向自身"', '"CNAME cannot point to itself"'),
        ('"CNAME 配置形成循环"', '"CNAME configuration creates a cycle"'),
    ],
    "backend/service/ssl_certificate.go": [
        ('"证书名称不能为空"', '"certificate name is required"'),
        ('"上传证书"', '"Upload Certificate"'),
        ('"证书已过期，禁止上传"', '"expired certificate cannot be uploaded"'),
        ('"证书域名 %s 不属于公网主域名 %s"', '"certificate domain %s does not belong to public main domain %s"'),
        ('"主域名和证书域名不能为空"', '"main domain and certificate domain are required"'),
        ('"第一阶段仅支持单域名和泛域名证书"', '"the current phase supports only single-domain and wildcard certificates"'),
        ('"泛域名证书必须使用 *.example.com 格式"', '"wildcard certificate must use the *.example.com format"'),
        ('"单域名证书不能包含通配符"', '"single-domain certificate cannot contain a wildcard"'),
        ('"证书域名不属于所选主域名"', '"certificate domain does not belong to the selected main domain"'),
        ('"当前主域名未关联可用 DNS 云账号，无法执行 DNS 所有权验证"', '"the current main domain has no available DNS cloud account for DNS ownership validation"'),
        ('"ACME 联系邮箱未配置"', '"ACME contact email is not configured"'),
        ('"申请证书"', '"Apply Certificate"'),
        ('"没有可用的 DNS 云账号"', '"no available DNS cloud account"'),
        ('"不支持的证书任务类型"', '"unsupported certificate task type"'),
        ('"只有平台 ACME 申请的证书支持续签"', '"only certificates issued by the platform ACME workflow support renewal"'),
        ('"该证书已有申请或续签任务正在执行"', '"an issuance or renewal task is already running for this certificate"'),
        ('"该证书已有相同任务正在执行"', '"an identical task is already running for this certificate"'),
        ('"证书任务异常: %v"', '"certificate task panic: %v"'),
        ('"未知证书任务"', '"unknown certificate task"'),
        ('"同步证书"', '"Synchronize Certificate"'),
        ('"云端证书缺少证书 ID"', '"cloud certificate is missing a certificate ID"'),
        ('"当前证书没有可上传的证书内容和私钥"', '"the current certificate has no certificate body and private key available for upload"'),
        ('"同步云端"', '"Synchronize to Cloud"'),
        ('"提前续签天数必须在 1 到 90 天之间"', '"renew-before days must be between 1 and 90"'),
        ('"只有平台 ACME 证书支持自动续签"', '"only platform ACME certificates support automatic renewal"'),
        ('"修改自动续签"', '"Update Automatic Renewal"'),
        ('"删除证书"', '"Delete Certificate"'),
        ('"未过期的泛域名证书禁止删除"', '"an unexpired wildcard certificate cannot be deleted"'),
        ('"删除前刷新公网 DNS 失败: %w"', '"failed to refresh public DNS before deletion: %w"'),
        ('"该证书对应域名当前仍存在公网 DNS 解析记录，请先删除相关解析记录后再删除证书"', '"public DNS records still exist for this certificate domain; delete the related records before deleting the certificate"'),
        ('"云端删除失败，本地证书已保留: %w"', '"cloud deletion failed; local certificate was retained: %w"'),
        ('"删除平台及云端证书"', '"Delete Platform and Cloud Certificate"'),
        ('"该证书没有保存在平台的 Private Key"', '"the certificate has no private key stored by the platform"'),
        ('"不支持的证书下载类型"', '"unsupported certificate download type"'),
        ('action := "下载证书"', 'action := "Download Certificate"'),
        ('action = "下载 Private Key"', 'action = "Download Private Key"'),
        ('"Certificate PEM 格式不正确"', '"invalid certificate PEM format"'),
        ('"解析证书失败: %w"', '"failed to parse certificate: %w"'),
        ('"Certificate 与 Private Key 不匹配"', '"certificate and private key do not match"'),
        ('"证书不包含可识别的 CN 或 SAN 域名"', '"certificate contains no recognizable CN or SAN domain"'),
        ('"Private Key PEM 格式不正确"', '"invalid private-key PEM format"'),
        ('"不支持或无法解析的 Private Key"', '"unsupported or unparseable private key"'),
        ('"DNS 云账号已停用"', '"DNS cloud account is disabled"'),
        ('"证书域名不能为空"', '"certificate domain is required"'),
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
            preview = "; ".join(f"{n}: {line.strip()}" for n, line in remaining[:10])
            failures.append(f"{relative_path} still contains Han characters: {preview}")
    print(f"batch4 localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
