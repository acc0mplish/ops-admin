#!/usr/bin/env python3
from __future__ import annotations

import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WEB_SRC = ROOT / "web" / "src"
CATALOG_PATH = WEB_SRC / "utils" / "english-hardcoding-i18n.js"

MESSAGES = {
    "newDatabaseNameRequired": ("새 데이터베이스 이름을 입력하십시오.", "Enter a new database name."),
    "sqlExecutionCompleted": ("SQL 실행을 완료했습니다.", "SQL execution completed."),
    "sqlRequired": ("실행할 SQL을 입력하십시오.", "Enter SQL to execute."),
    "tableNoPrimaryKeyDirectEdit": ("현재 테이블에 Primary Key가 없어 결과를 직접 편집할 수 없습니다.", "The current table has no primary key, so direct result editing is unavailable."),
    "tableNoPrimaryKeyDelete": ("현재 테이블에 Primary Key가 없어 행을 삭제할 수 없습니다.", "The current table has no primary key, so rows cannot be deleted."),
    "selectDatabase": ("데이터베이스를 선택하십시오.", "Select a database."),
    "selectTable": ("테이블을 선택하십시오.", "Select a table."),
    "selectSchema": ("Schema를 선택하십시오.", "Select a schema."),
    "queryHistory": ("Query 이력", "Query History"),
    "executionPlan": ("실행 계획", "Execution Plan"),
    "resultSet": ("결과 집합", "Result Set"),
    "newQuery": ("새 Query", "New Query"),
    "saveQuery": ("Query 저장", "Save Query"),
    "pipelineNameRequired": ("파이프라인 이름을 입력하십시오.", "Enter a pipeline name."),
    "pipelineCreated": ("파이프라인을 생성했습니다.", "Pipeline created."),
    "pipelineSaved": ("파이프라인을 저장했습니다.", "Pipeline saved."),
    "pipelineDeleted": ("파이프라인을 삭제했습니다.", "Pipeline deleted."),
    "noPipelineForApplication": ("선택한 애플리케이션에 파이프라인이 없습니다.", "The selected application has no pipelines."),
    "selectApplication": ("애플리케이션을 선택하십시오.", "Select an application."),
    "selectEnvironment": ("환경을 선택하십시오.", "Select an environment."),
    "buildTask": ("빌드 작업", "Build Task"),
    "buildHistory": ("빌드 이력", "Build History"),
    "imageRegistry": ("이미지 Registry", "Image Registry"),
    "sourceRepository": ("소스 저장소", "Source Repository"),
    "runtimeEnvironment": ("Runtime 환경", "Runtime Environment"),
    "deploymentStrategy": ("배포 전략", "Deployment Strategy"),
    "triggerMode": ("실행 방식", "Trigger Mode"),
    "manualTrigger": ("수동 실행", "Manual Trigger"),
    "webhookTrigger": ("Webhook 실행", "Webhook Trigger"),
    "createPipeline": ("파이프라인 생성", "Create Pipeline"),
    "editPipeline": ("파이프라인 수정", "Edit Pipeline"),
    "deletePipeline": ("파이프라인 삭제", "Delete Pipeline"),
    "runPipeline": ("파이프라인 실행", "Run Pipeline"),
    "dashboardName": ("대시보드 이름", "Dashboard Name"),
    "panelTitle": ("패널 제목", "Panel Title"),
    "inspectionReportRefreshed": ("점검 보고서를 새로고침했습니다.", "Inspection report refreshed."),
    "dashboardSaved": ("대시보드를 저장했습니다.", "Dashboard saved."),
    "dashboardDeleted": ("대시보드를 삭제했습니다.", "Dashboard deleted."),
    "panelSaved": ("패널을 저장했습니다.", "Panel saved."),
    "panelDeleted": ("패널을 삭제했습니다.", "Panel deleted."),
    "dataSource": ("Data Source", "Data Source"),
    "query": ("Query", "Query"),
    "visualization": ("시각화", "Visualization"),
    "threshold": ("임계값", "Threshold"),
    "unit": ("단위", "Unit"),
    "legend": ("범례", "Legend"),
    "timeRange": ("시간 범위", "Time Range"),
    "promqlConditions": ("PromQL 조건", "PromQL Conditions"),
    "notificationRule": ("통지 규칙", "Notification Rule"),
    "alertRule": ("Alert Rule", "Alert Rule"),
    "evaluationInterval": ("평가 주기", "Evaluation Interval"),
    "forDuration": ("지속 시간", "For Duration"),
    "severity": ("심각도", "Severity"),
    "summary": ("요약", "Summary"),
    "description": ("설명", "Description"),
    "queryExpression": ("Query 식", "Query Expression"),
    "noData": ("데이터 없음", "No Data"),
    "errorHandling": ("오류 처리", "Error Handling"),
    "templateVariables": ("Template Variable", "Template Variables"),
    "addCondition": ("조건 추가", "Add Condition"),
    "testQuery": ("Query 테스트", "Test Query"),
}

FILE_MESSAGES = {
    "views/assets/DatabaseWorkbench.vue": {
        "새 Database 이름을 입력하십시오.": "newDatabaseNameRequired",
        "SQL 실행 완료": "sqlExecutionCompleted",
        "실행할 SQL을 입력하십시오.": "sqlRequired",
        "현재 Table에는 Primary Key가 없어 direct editing 할 수 없습니다.": "tableNoPrimaryKeyDirectEdit",
        "현재 Table에는 Primary Key가 없어 직접 편집할 수 없습니다.": "tableNoPrimaryKeyDirectEdit",
        "현재 Table에는 Primary Key가 없어 Row를 삭제할 수 없습니다.": "tableNoPrimaryKeyDelete",
        "Database를 선택하십시오.": "selectDatabase",
        "Table을 선택하십시오.": "selectTable",
        "Schema를 선택하십시오.": "selectSchema",
        "Query History": "queryHistory",
        "Execution Plan": "executionPlan",
        "Result Set": "resultSet",
        "New Query": "newQuery",
        "Save Query": "saveQuery",
    },
    "views/applications/AppPipelineCenter.vue": {
        "Pipeline name is required.": "pipelineNameRequired",
        "Pipeline이 생성되었습니다.": "pipelineCreated",
        "Pipeline이 저장되었습니다.": "pipelineSaved",
        "Pipeline을 삭제했습니다.": "pipelineDeleted",
        "선택한 Application에 Pipeline이 없습니다.": "noPipelineForApplication",
        "Application을 선택하십시오.": "selectApplication",
        "Environment를 선택하십시오.": "selectEnvironment",
        "Build Task": "buildTask",
        "Build History": "buildHistory",
        "Image Registry": "imageRegistry",
        "Source Repository": "sourceRepository",
        "Runtime Environment": "runtimeEnvironment",
        "Deployment Strategy": "deploymentStrategy",
        "Trigger Mode": "triggerMode",
        "Manual Trigger": "manualTrigger",
        "Webhook Trigger": "webhookTrigger",
        "Create Pipeline": "createPipeline",
        "Edit Pipeline": "editPipeline",
        "Delete Pipeline": "deletePipeline",
        "Run Pipeline": "runPipeline",
    },
    "views/monitor/MonitorDashboard.vue": {
        "Dashboard 이름": "dashboardName",
        "Panel 제목": "panelTitle",
        "Inspection Report를 새로고침했습니다.": "inspectionReportRefreshed",
        "Dashboard를 저장했습니다.": "dashboardSaved",
        "Dashboard를 삭제했습니다.": "dashboardDeleted",
        "Panel을 저장했습니다.": "panelSaved",
        "Panel을 삭제했습니다.": "panelDeleted",
        "Data Source": "dataSource",
        "Query": "query",
        "Visualization": "visualization",
        "Threshold": "threshold",
        "Unit": "unit",
        "Legend": "legend",
        "Time Range": "timeRange",
    },
    "views/monitor/MonitorAlertRule.vue": {
        "PromQL Conditions": "promqlConditions",
        "Notification Rule": "notificationRule",
        "Alert Rule": "alertRule",
        "Data Source": "dataSource",
        "Evaluation Interval": "evaluationInterval",
        "For Duration": "forDuration",
        "Severity": "severity",
        "Summary": "summary",
        "Description": "description",
        "Query Expression": "queryExpression",
        "No Data": "noData",
        "Error Handling": "errorHandling",
        "Template Variables": "templateVariables",
        "Add Condition": "addCondition",
        "Test Query": "testQuery",
    },
}

STATIC_ATTRIBUTES = (
    "label", "title", "placeholder", "description", "empty-text", "alt", "aria-label",
    "content", "header", "loading-text", "no-data-text", "confirm-button-text", "cancel-button-text",
)


def quote(value: str) -> str:
    return value.replace("\\", "\\\\").replace("'", "\\'")


def append_catalog_entries(source: str) -> str:
    missing = [key for key in MESSAGES if not re.search(rf"^\s*{re.escape(key)}\s*:", source, re.M)]
    if not missing:
        return source

    ko_start = source.index("const ko = {")
    en_start = source.index("const en = {")
    function_start = source.index("export function uiT")

    ko_block = source[ko_start:en_start]
    en_block = source[en_start:function_start]
    ko_close = ko_block.rfind("}")
    en_close = en_block.rfind("}")

    ko_lines = "".join(f"  {key}: '{quote(MESSAGES[key][0])}',\n" for key in missing)
    en_lines = "".join(f"  {key}: '{quote(MESSAGES[key][1])}',\n" for key in missing)

    ko_block = ko_block[:ko_close] + ko_lines + ko_block[ko_close:]
    en_block = en_block[:en_close] + en_lines + en_block[en_close:]
    return source[:ko_start] + ko_block + en_block + source[function_start:]


def import_path_for(path: Path) -> str:
    relative = os.path.relpath(CATALOG_PATH.with_suffix(""), path.parent).replace(os.sep, "/")
    return relative if relative.startswith(".") else "./" + relative


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

    text_node = re.compile(rf"(?P<before>>\s*){escaped}(?P<after>\s*<)")
    source, changed = text_node.subn(lambda m: f"{m.group('before')}{{{{ uiT('{key}') }}}}{m.group('after')}", source)
    count += changed

    attribute_names = "|".join(re.escape(name) for name in STATIC_ATTRIBUTES)
    static_attr = re.compile(rf"(?<![:@\w-])(?P<name>{attribute_names})=(?P<quote>['\"]){escaped}(?P=quote)")
    source, changed = static_attr.subn(lambda m: f":{m.group('name')}=\"uiT('{key}')\"", source)
    count += changed

    bound_attr = re.compile(rf"(?P<prefix>[:@][\w-]+=)(?P<outer>['\"])(?P<inner>['\"]){escaped}(?P=inner)(?P=outer)")
    source, changed = bound_attr.subn(lambda m: f"{m.group('prefix')}\"uiT('{key}')\"", source)
    count += changed

    for literal_quote in ("'", '"', "`"):
        token = f"{literal_quote}{old}{literal_quote}"
        occurrences = source.count(token)
        if occurrences:
            source = source.replace(token, f"uiT('{key}')")
            count += occurrences

    return source, count


def main() -> int:
    if not CATALOG_PATH.exists():
        raise FileNotFoundError(CATALOG_PATH)
    catalog = CATALOG_PATH.read_text(encoding="utf-8")
    CATALOG_PATH.write_text(append_catalog_entries(catalog), encoding="utf-8")

    total = 0
    changed_files = 0
    for relative, mapping in FILE_MESSAGES.items():
        path = WEB_SRC / relative
        original = path.read_text(encoding="utf-8")
        updated = original
        file_count = 0
        for old, key in mapping.items():
            updated, count = replace_user_string(updated, old, key)
            file_count += count
        if file_count:
            updated = ensure_import(updated, path)
            path.write_text(updated, encoding="utf-8")
            changed_files += 1
            total += file_count
            print(f"{relative}: {file_count} replacement(s)")
        else:
            print(f"{relative}: no matching target strings")

    print(f"Updated {changed_files} files with {total} mixed or direct English UI replacement(s).")
    if total == 0:
        raise SystemExit("No target strings were replaced; source patterns may have drifted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
