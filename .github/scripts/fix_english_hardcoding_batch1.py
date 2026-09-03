#!/usr/bin/env python3
from __future__ import annotations

import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WEB_SRC = ROOT / "web" / "src"
CATALOG_PATH = WEB_SRC / "utils" / "english-hardcoding-i18n.js"

MESSAGES = {
    "cloudNativeDelivery": ("클라우드 네이티브 배포", "Cloud Native Delivery"),
    "cicdPipeline": ("CI/CD 파이프라인", "CI/CD Pipeline"),
    "application": ("애플리케이션", "Application"),
    "environment": ("환경", "Environment"),
    "techStack": ("기술 스택", "Tech Stack"),
    "keyword": ("키워드", "Keyword"),
    "pipeline": ("파이프라인", "Pipeline"),
    "containerImageDelivery": ("컨테이너 이미지 배포", "Container Image Delivery"),
    "serviceAsset": ("서비스 자산", "Service Asset"),
    "intelligentOperationsCenter": ("Ops Admin 지능형 운영 센터", "Ops Admin Intelligent Operations Center"),
    "activeAlert": ("활성 Alert", "Active Alert"),
    "authRate": ("인증 성공률", "Authentication Rate"),
    "physicalHost": ("물리 호스트", "Physical Host"),
    "resourceSituation": ("리소스 현황", "Resource Situation"),
    "topFive": ("TOP 5", "TOP 5"),
    "monitoringDashboard": ("모니터링 대시보드", "Monitoring Dashboard"),
    "inspectionReportPdfExport": ("점검 보고서 PDF 내보내기", "Inspection Report PDF Export"),
    "inspectionPanel": ("점검 패널", "Inspection Panel"),
    "panel": ("패널", "Panel"),
    "monitoringOverview": ("모니터링 개요", "Monitoring Overview"),
    "metricExplorer": ("Metric 탐색기", "Metric Explorer"),
    "metricTrend": ("Metric 추세", "Metric Trend"),
    "dnsDiagnostics": ("DNS 진단", "DNS Diagnostics"),
    "domain": ("도메인", "Domain"),
    "type": ("유형", "Type"),
    "dnsServer": ("DNS 서버", "DNS Server"),
    "dnsChangeAudit": ("DNS 변경 감사", "DNS Change Audit"),
    "provider": ("Provider", "Provider"),
    "zone": ("Zone", "Zone"),
    "tlsCertificateDetail": ("TLS 인증서 상세", "TLS Certificate Detail"),
    "certificatePem": ("인증서 PEM", "Certificate PEM"),
    "certificateChain": ("인증서 체인", "Certificate Chain"),
    "integrationNavigation": ("통합 탐색", "Integration Navigation"),
    "groupDirectory": ("그룹 디렉터리", "Group Directory"),
    "groupWorkspace": ("그룹 워크스페이스", "Group Workspace"),
    "sharedNavigation": ("공유 탐색", "Shared Navigation"),
    "dbmsWorkbench": ("DBMS 워크벤치", "DBMS Workbench"),
    "databaseConnection": ("데이터베이스 연결", "Database Connection"),
    "databaseTableStructure": ("데이터베이스 / 테이블 구조", "Database / Table Structure"),
    "ready": ("준비됨", "Ready"),
    "browser": ("브라우저", "Browser"),
    "assetControl": ("자산 제어", "Asset Control"),
    "assetInventory": ("자산 인벤토리", "Asset Inventory"),
    "publicDnsCredentials": ("Public DNS Credential", "Public DNS Credentials"),
    "dnsRuntimeSettings": ("DNS Runtime 설정", "DNS Runtime Settings"),
    "authoritativeRecords": ("Authoritative Record", "Authoritative Records"),
    "privateDnsZones": ("Private DNS Zone", "Private DNS Zones"),
    "publicDnsInventory": ("Public DNS 인벤토리", "Public DNS Inventory"),
    "publicDnsRecords": ("Public DNS Record", "Public DNS Records"),
    "tlsCertificateInventory": ("TLS 인증서 인벤토리", "TLS Certificate Inventory"),
    "aiOperationsCopilot": ("AI 운영 Copilot", "AI Operations Copilot"),
    "conversationMemory": ("대화 Memory", "Conversation Memory"),
    "localMarkdownKnowledge": ("로컬 Markdown Knowledge", "Local Markdown Knowledge"),
    "openaiCompatibleModels": ("OpenAI 호환 Model", "OpenAI-compatible Models"),
    "operationToolRegistry": ("운영 Tool Registry", "Operations Tool Registry"),
}

FILE_MESSAGES = {
    "views/applications/AppPipelineCenter.vue": {
        "Cloud Native Delivery": "cloudNativeDelivery",
        "CI/CD Pipeline": "cicdPipeline",
        "Application": "application",
        "Environment": "environment",
        "Tech Stack": "techStack",
        "Keyword": "keyword",
        "Pipeline": "pipeline",
        "CONTAINER IMAGE DELIVERY": "containerImageDelivery",
        "SERVICE ASSET": "serviceAsset",
    },
    "views/monitor/MonitorCommandCenter.vue": {
        "OPS ADMIN INTELLIGENT OPERATIONS CENTER": "intelligentOperationsCenter",
        "ACTIVE ALERT": "activeAlert",
        "AUTH RATE": "authRate",
        "PHYSICAL HOST": "physicalHost",
        "RESOURCE SITUATION": "resourceSituation",
        "TOP 5": "topFive",
    },
    "views/monitor/MonitorDashboard.vue": {
        "Monitoring Dashboard": "monitoringDashboard",
        "Inspection Report PDF Export": "inspectionReportPdfExport",
        "Inspection Panel": "inspectionPanel",
        "Panel": "panel",
        "MONITORING OVERVIEW": "monitoringOverview",
        "METRIC EXPLORER": "metricExplorer",
        "Metric trend": "metricTrend",
    },
    "views/domains/DNSQueryTest.vue": {
        "DNS DIAGNOSTICS": "dnsDiagnostics",
        "Domain": "domain",
        "Type": "type",
        "DNS Server": "dnsServer",
    },
    "views/domains/DNSAudit.vue": {
        "DNS CHANGE AUDIT": "dnsChangeAudit",
        "Provider": "provider",
        "Zone": "zone",
    },
    "views/domains/SSLCertificateDetail.vue": {
        "TLS CERTIFICATE DETAIL": "tlsCertificateDetail",
        "Certificate PEM": "certificatePem",
        "Certificate Chain": "certificateChain",
    },
    "views/integration/IntegrationNavigation.vue": {
        "INTEGRATION NAVIGATION": "integrationNavigation",
        "GROUP DIRECTORY": "groupDirectory",
        "GROUP WORKSPACE": "groupWorkspace",
        "SHARED NAVIGATION": "sharedNavigation",
    },
    "views/assets/DatabaseWorkbench.vue": {
        "DBMS WORKBENCH": "dbmsWorkbench",
        "Database Connection": "databaseConnection",
        "Database / Table Structure": "databaseTableStructure",
        "Ready": "ready",
        "Browser": "browser",
    },
    "views/assets/AssetOverview.vue": {
        "ASSET CONTROL": "assetControl",
        "ASSET INVENTORY": "assetInventory",
    },
    "views/domains/DNSAccounts.vue": {
        "PUBLIC DNS CREDENTIALS": "publicDnsCredentials",
    },
    "views/domains/InternalDNSSettings.vue": {
        "DNS RUNTIME SETTINGS": "dnsRuntimeSettings",
    },
    "views/domains/InternalRecords.vue": {
        "AUTHORITATIVE RECORDS": "authoritativeRecords",
    },
    "views/domains/InternalZones.vue": {
        "PRIVATE DNS ZONES": "privateDnsZones",
    },
    "views/domains/PublicDomains.vue": {
        "PUBLIC DNS INVENTORY": "publicDnsInventory",
    },
    "views/domains/PublicRecords.vue": {
        "PUBLIC DNS RECORDS": "publicDnsRecords",
    },
    "views/domains/SSLCertificates.vue": {
        "TLS CERTIFICATE INVENTORY": "tlsCertificateInventory",
    },
    "views/integration/AIAssistantChat.vue": {
        "AI OPERATIONS COPILOT": "aiOperationsCopilot",
    },
    "views/integration/AIConversations.vue": {
        "CONVERSATION MEMORY": "conversationMemory",
    },
    "views/integration/AIKnowledgeBase.vue": {
        "LOCAL MARKDOWN KNOWLEDGE": "localMarkdownKnowledge",
    },
    "views/integration/AIModels.vue": {
        "OPENAI COMPATIBLE MODELS": "openaiCompatibleModels",
        "OPENAI-COMPATIBLE MODELS": "openaiCompatibleModels",
    },
    "views/integration/AITools.vue": {
        "OPERATION TOOL REGISTRY": "operationToolRegistry",
        "OPERATIONS TOOL REGISTRY": "operationToolRegistry",
    },
}

STATIC_ATTRIBUTES = (
    "label", "title", "placeholder", "description", "empty-text", "alt", "aria-label",
    "content", "header", "loading-text", "no-data-text", "confirm-button-text", "cancel-button-text",
)


def render_catalog() -> str:
    def render_dict(index: int) -> str:
        lines = []
        for key, values in MESSAGES.items():
            value = values[index].replace("\\", "\\\\").replace("'", "\\'")
            lines.append(f"  {key}: '{value}',")
        return "\n".join(lines)

    return f"""import {{ currentLocale }} from './i18n-runtime'\n\nconst ko = {{\n{render_dict(0)}\n}}\n\nconst en = {{\n{render_dict(1)}\n}}\n\nexport function uiT(key, params = {{}}) {{\n  const dictionary = currentLocale.value === 'en-US' ? en : ko\n  let text = dictionary[key] || en[key] || key\n  Object.entries(params).forEach(([name, value]) => {{\n    text = text.replaceAll(`{{${{name}}}}`, String(value))\n  }})\n  return text\n}}\n"""


def import_path_for(path: Path) -> str:
    relative = os.path.relpath(CATALOG_PATH.with_suffix(""), path.parent)
    return relative.replace(os.sep, "/") if relative.startswith(".") else "./" + relative.replace(os.sep, "/")


def ensure_import(source: str, path: Path) -> str:
    if "english-hardcoding-i18n" in source:
        return source
    marker = "<script setup>\n"
    if marker not in source:
        raise RuntimeError(f"{path.relative_to(ROOT)} does not use <script setup>")
    return source.replace(marker, marker + f"import {{ uiT }} from '{import_path_for(path)}'\n", 1)


def replace_user_string(source: str, old: str, key: str) -> tuple[str, int]:
    count = 0
    escaped = re.escape(old)

    # Visible text nodes in Vue templates.
    pattern = re.compile(rf"(?P<before>>\s*){escaped}(?P<after>\s*<)")
    source, replaced = pattern.subn(lambda m: f"{m.group('before')}{{{{ uiT('{key}') }}}}{m.group('after')}", source)
    count += replaced

    # Static user-facing Vue attributes.
    attribute_names = "|".join(re.escape(name) for name in STATIC_ATTRIBUTES)
    pattern = re.compile(rf"(?<![:@\w-])(?P<name>{attribute_names})=(?P<quote>['\"]){escaped}(?P=quote)")
    source, replaced = pattern.subn(lambda m: f":{m.group('name')}=\"uiT('{key}')\"", source)
    count += replaced

    # Bound attributes containing only a string literal.
    pattern = re.compile(rf"(?P<prefix>[:@][\w-]+=)(?P<outer>['\"])(?P<inner>['\"]){escaped}(?P=inner)(?P=outer)")
    source, replaced = pattern.subn(lambda m: f"{m.group('prefix')}\"uiT('{key}')\"", source)
    count += replaced

    # Script literals, including object labels, dialog titles and message fallbacks.
    for quote in ("'", '"', "`"):
        token = f"{quote}{old}{quote}"
        occurrences = source.count(token)
        if occurrences:
            source = source.replace(token, f"uiT('{key}')")
            count += occurrences

    return source, count


def main() -> int:
    CATALOG_PATH.write_text(render_catalog(), encoding="utf-8")
    total = 0
    changed_files = 0

    for relative, mapping in FILE_MESSAGES.items():
        path = WEB_SRC / relative
        if not path.exists():
            raise FileNotFoundError(path)
        original = path.read_text(encoding="utf-8")
        updated = original
        file_replacements = 0
        for old, key in mapping.items():
            updated, count = replace_user_string(updated, old, key)
            file_replacements += count
        if file_replacements:
            updated = ensure_import(updated, path)
            path.write_text(updated, encoding="utf-8")
            changed_files += 1
            total += file_replacements
            print(f"{relative}: {file_replacements} replacement(s)")
        else:
            print(f"{relative}: no matching direct user-visible strings")

    print(f"Updated {changed_files} files with {total} direct English UI replacement(s).")
    if total == 0:
        raise SystemExit("No target strings were replaced; source patterns may have drifted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
