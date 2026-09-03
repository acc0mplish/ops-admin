#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import html
import json
import re
from collections import Counter
from dataclasses import asdict, dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

SOURCE_ROOTS = (ROOT / "web" / "src", ROOT / "backend")
SOURCE_SUFFIXES = {".vue", ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs", ".go"}

EXCLUDED_PARTS = {
    "node_modules", "vendor", "dist", "build", "coverage", ".git", "generated",
    "locales", "translations", "fixtures", "testdata", "__tests__",
}
EXCLUDED_FILE_PATTERNS = (
    re.compile(r"(?:^|[._-])(?:test|spec)(?:[._-]|$)", re.I),
    re.compile(r"^(?:en|ko)(?:[-_][A-Z]{2})?\.(?:js|ts|json)$", re.I),
)

HAN_RE = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF]")
LATIN_RE = re.compile(r"[A-Za-z]")
LATIN_WORD_RE = re.compile(r"[A-Za-z][A-Za-z0-9+.#/-]*")
SPACE_RE = re.compile(r"\s+")
INTERPOLATION_RE = re.compile(r"(?:\$\{.*?\}|\{\{.*?\}\})", re.S)

TECHNICAL_EXACT = {
    "api", "http", "https", "ssh", "ssl", "tls", "tcp", "udp", "dns", "ip", "url", "uri",
    "sql", "ddl", "dml", "json", "yaml", "yml", "xml", "csv", "tsv", "html", "css", "javascript",
    "typescript", "go", "python", "bash", "shell", "git", "svn", "docker", "helm", "promql", "cron",
    "cpu", "gpu", "iops", "qps", "p50", "p90", "p95", "p99", "uuid", "uid", "id", "jwt", "oauth",
    "kubernetes", "k8s", "redis", "mysql", "postgresql", "postgres", "mongodb", "prometheus",
    "victoriametrics", "victorialogs", "grafana", "elasticsearch", "opensearch", "nginx", "linux",
    "deployment", "statefulset", "daemonset", "replicaset", "cronjob", "job", "pod", "service", "ingress",
    "gateway", "namespace", "node", "secret", "configmap", "persistentvolume", "persistentvolumeclaim",
    "pv", "pvc", "storageclass", "container", "image", "webhook", "grpc", "rest", "tcp", "udp",
    "get", "post", "put", "patch", "delete", "options", "head", "true", "false", "null", "undefined",
    "success", "failed", "failure", "pending", "running", "completed", "enabled", "disabled", "active",
    "inactive", "unknown", "warning", "error", "info", "debug", "trace", "read", "write", "admin",
}

USER_ATTRIBUTE_KEYS = {
    "label", "title", "placeholder", "description", "content", "empty-text", "emptyText",
    "confirm-button-text", "cancel-button-text", "loading-text", "no-data-text", "header",
    "tooltip", "aria-label", "alt", "button-text", "message",
}
OBJECT_FIELD_KEYS = {
    "label", "title", "placeholder", "description", "message", "summary", "statusText",
    "tooltip", "emptyText", "helpText", "hint", "caption", "displayName",
}

STRING_LITERAL_RE = re.compile(
    r"(?P<quote>['\"`])(?P<body>(?:\\.|(?!\1).)*?)(?P=quote)",
    re.S,
)
ATTRIBUTE_RE = re.compile(
    r"(?<![:@\w-])(?P<key>" + "|".join(re.escape(k) for k in sorted(USER_ATTRIBUTE_KEYS, key=len, reverse=True)) +
    r")\s*=\s*(?P<quote>['\"])(?P<body>(?:\\.|(?!\2).)*?)(?P=quote)",
    re.S,
)
OBJECT_FIELD_RE = re.compile(
    r"\b(?P<key>" + "|".join(re.escape(k) for k in sorted(OBJECT_FIELD_KEYS, key=len, reverse=True)) +
    r")\s*:\s*(?P<quote>['\"`])(?P<body>(?:\\.|(?!\2).)*?)(?P=quote)",
    re.S,
)
GO_FIELD_RE = re.compile(
    r"\b(?P<key>Label|Title|Description|Message|Summary|StatusText|HelpText|Hint|Caption|DisplayName)\s*:\s*"
    r"(?P<quote>['\"])(?P<body>(?:\\.|(?!\2).)*?)(?P=quote)",
    re.S,
)
CALL_PATTERNS = [
    ("ui-message", "high", re.compile(
        r"(?:ElMessage\.(?:success|warning|error|info)|ElNotification(?:\.(?:success|warning|error|info))?|"
        r"window\.(?:alert|confirm|prompt)|\balert|\bconfirm|\bprompt)\s*\(\s*"
        r"(?P<quote>['\"`])(?P<body>(?:\\.|(?!\1).)*?)(?P=quote)", re.S)),
    ("ui-dialog", "high", re.compile(
        r"ElMessageBox\.(?:alert|confirm|prompt)\s*\(\s*"
        r"(?P<quote>['\"`])(?P<body>(?:\\.|(?!\1).)*?)(?P=quote)", re.S)),
    ("js-error", "high", re.compile(
        r"(?:throw\s+new\s+Error|new\s+Error|Promise\.reject)\s*\(\s*"
        r"(?P<quote>['\"`])(?P<body>(?:\\.|(?!\1).)*?)(?P=quote)", re.S)),
    ("go-error", "high", re.compile(
        r"(?:errors\.New|fmt\.Errorf|status\.Errorf?|http\.Error|AbortWithStatusJSON|JSONError|BadRequest|"
        r"Unauthorized|Forbidden|NotFound|InternalServerError)\s*\([^\n]*?"
        r"(?P<quote>['\"])(?P<body>(?:\\.|(?!\1).)*?)(?P=quote)", re.S)),
]

IMPORT_LINE_RE = re.compile(r"^\s*(?:import|export\s+.*?from|require\s*\(|package\s+|go:embed)", re.I)
COMMENT_LINE_RE = re.compile(r"^\s*(?://|/\*|\*|<!--)")
URL_RE = re.compile(r"^(?:https?|wss?|ssh|git)://", re.I)
PATH_RE = re.compile(r"^(?:\.?\.?/|/api/|/apis/|/v\d+/|[A-Za-z]:\\)")
IDENTIFIER_RE = re.compile(r"^[A-Za-z_$][A-Za-z0-9_$]*(?:[._:/-][A-Za-z0-9_$@{}-]+)+$")
SNAKE_OR_KEY_RE = re.compile(r"^[a-z][a-z0-9]*(?:[_-][a-z0-9]+)+$")
MIME_RE = re.compile(r"^[a-z][a-z0-9.+-]*/[a-z0-9.+-]+$", re.I)
HEX_RE = re.compile(r"^(?:#[0-9a-f]{3,8}|0x[0-9a-f]+)$", re.I)
FORMAT_ONLY_RE = re.compile(r"^(?:[%{}$#:\-+./_\s]|\\[nrt])+$")
SQL_START_RE = re.compile(r"^(?:select|insert|update|delete|create|alter|drop|grant|revoke|with|show|describe|explain)\b", re.I)
CODEISH_RE = re.compile(r"(?:=>|===|!==|&&|\|\||\bfunc\b|\bconst\b|\bvar\b|\blet\b|\breturn\b)")

@dataclass(frozen=True)
class Finding:
    severity: str
    kind: str
    file: str
    line: int
    text: str
    context: str


def iter_source_files() -> list[Path]:
    files: list[Path] = []
    for root in SOURCE_ROOTS:
        if not root.exists():
            continue
        for path in root.rglob("*"):
            if not path.is_file() or path.suffix.lower() not in SOURCE_SUFFIXES:
                continue
            rel = path.relative_to(ROOT)
            if any(part in EXCLUDED_PARTS for part in rel.parts):
                continue
            if any(pattern.search(path.name) for pattern in EXCLUDED_FILE_PATTERNS):
                continue
            files.append(path)
    return sorted(files)


def line_number(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def line_context(text: str, offset: int, limit: int = 240) -> str:
    start = text.rfind("\n", 0, offset) + 1
    end = text.find("\n", offset)
    if end < 0:
        end = len(text)
    value = SPACE_RE.sub(" ", text[start:end].strip())
    return value[:limit]


def normalize_candidate(raw: str) -> str:
    value = html.unescape(raw)
    value = INTERPOLATION_RE.sub("{…}", value)
    value = value.replace("\\n", " ").replace("\\r", " ").replace("\\t", " ")
    value = value.replace("\\\"", '"').replace("\\'", "'")
    value = SPACE_RE.sub(" ", value).strip(" \t\r\n|•·:;,-")
    return value


def is_translation_key_context(context: str, raw: str) -> bool:
    escaped = re.escape(raw)
    return bool(re.search(rf"\b(?:t|i18n\.t|translate)\s*\(\s*['\"`]{escaped}['\"`]", context))


def is_technical_only(value: str, *, explicit_surface: bool) -> bool:
    stripped = value.strip()
    lowered = stripped.lower()
    if not stripped or len(stripped) > 600:
        return True
    if not LATIN_RE.search(stripped):
        return True
    if FORMAT_ONLY_RE.fullmatch(stripped):
        return True
    if URL_RE.search(stripped) or PATH_RE.search(stripped) or MIME_RE.fullmatch(stripped) or HEX_RE.fullmatch(stripped):
        return True
    if lowered in TECHNICAL_EXACT:
        return True
    if SQL_START_RE.search(stripped):
        return True
    if CODEISH_RE.search(stripped) and not explicit_surface:
        return True
    if IDENTIFIER_RE.fullmatch(stripped) or SNAKE_OR_KEY_RE.fullmatch(stripped):
        return True
    if re.fullmatch(r"[A-Z0-9_.:/@+-]{2,}", stripped) and lowered not in {"ok", "yes", "no"}:
        return True
    if re.fullmatch(r"\d+(?:\.\d+){1,3}", stripped):
        return True
    if re.fullmatch(r"[a-z]+(?:\.[a-z0-9_-]+){1,}", stripped):
        return True
    if not explicit_surface and len(LATIN_WORD_RE.findall(stripped)) < 2 and not HAN_RE.search(stripped):
        return True
    return False


def add_finding(
    findings: dict[tuple[str, int, str], Finding],
    path: Path,
    source: str,
    offset: int,
    raw: str,
    kind: str,
    severity: str,
    *,
    explicit_surface: bool,
) -> None:
    value = normalize_candidate(raw)
    context = line_context(source, offset)
    if COMMENT_LINE_RE.match(context) or IMPORT_LINE_RE.match(context):
        return
    if is_translation_key_context(context, raw):
        return
    if is_technical_only(value, explicit_surface=explicit_surface):
        return
    # Purely English or mixed Korean/English are both relevant. Chinese remnants are handled by another guard.
    if not LATIN_RE.search(value):
        return
    rel = path.relative_to(ROOT).as_posix()
    line = line_number(source, offset)
    key = (rel, line, value)
    candidate = Finding(severity=severity, kind=kind, file=rel, line=line, text=value, context=context)
    previous = findings.get(key)
    rank = {"high": 3, "medium": 2, "low": 1}
    if previous is None or rank[candidate.severity] > rank[previous.severity]:
        findings[key] = candidate


def scan_vue_template(path: Path, source: str, findings: dict[tuple[str, int, str], Finding]) -> None:
    match = re.search(r"<template(?:\s[^>]*)?>(?P<body>.*?)</template>", source, re.S | re.I)
    if not match:
        return
    body = match.group("body")
    base = match.start("body")
    body_without_comments = re.sub(r"<!--.*?-->", lambda m: " " * len(m.group(0)), body, flags=re.S)

    for attr in ATTRIBUTE_RE.finditer(body_without_comments):
        add_finding(
            findings, path, source, base + attr.start(), attr.group("body"),
            f"vue-attribute:{attr.group('key')}", "high", explicit_surface=True,
        )

    for text_match in re.finditer(r">(?P<body>[^<>{}\n]*[A-Za-z][^<>{}\n]*)<", body_without_comments):
        line = body_without_comments[max(0, text_match.start() - 80):text_match.end() + 80]
        if re.search(r"<(?:code|pre|script|style|svg|path|el-icon)\b", line, re.I):
            continue
        add_finding(
            findings, path, source, base + text_match.start("body"), text_match.group("body"),
            "vue-text", "high", explicit_surface=True,
        )


def scan_source(path: Path) -> list[Finding]:
    source = path.read_text(encoding="utf-8", errors="replace")
    findings: dict[tuple[str, int, str], Finding] = {}

    if path.suffix.lower() == ".vue":
        scan_vue_template(path, source, findings)

    for kind, severity, pattern in CALL_PATTERNS:
        for match in pattern.finditer(source):
            add_finding(
                findings, path, source, match.start(), match.group("body"),
                kind, severity, explicit_surface=True,
            )

    field_pattern = GO_FIELD_RE if path.suffix.lower() == ".go" else OBJECT_FIELD_RE
    for match in field_pattern.finditer(source):
        add_finding(
            findings, path, source, match.start(), match.group("body"),
            f"user-field:{match.group('key')}", "medium", explicit_surface=True,
        )

    # Catch remaining literals only on lines that strongly suggest user-visible output.
    hints = re.compile(
        r"\b(message|title|label|placeholder|description|tooltip|summary|caption|help|hint|"
        r"error|warning|success|confirm|prompt|button|statusText|displayName)\b",
        re.I,
    )
    for match in STRING_LITERAL_RE.finditer(source):
        context = line_context(source, match.start())
        if not hints.search(context):
            continue
        add_finding(
            findings, path, source, match.start(), match.group("body"),
            "contextual-string", "low", explicit_surface=False,
        )

    return sorted(findings.values(), key=lambda item: (item.line, item.text))


def write_tsv(path: Path, findings: list[Finding]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle, delimiter="\t")
        writer.writerow(["severity", "kind", "file", "line", "text", "context"])
        for item in findings:
            writer.writerow([item.severity, item.kind, item.file, item.line, item.text, item.context])


def write_json(path: Path, findings: list[Finding]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps([asdict(item) for item in findings], ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_markdown(path: Path, findings: list[Finding], scanned_files: int) -> None:
    severity_counts = Counter(item.severity for item in findings)
    file_counts = Counter(item.file for item in findings)
    kind_counts = Counter(item.kind for item in findings)
    lines = [
        "# English hardcoding audit",
        "",
        f"- Scanned source files: **{scanned_files}**",
        f"- Candidate findings: **{len(findings)}**",
        f"- High confidence: **{severity_counts['high']}**",
        f"- Medium confidence: **{severity_counts['medium']}**",
        f"- Low confidence: **{severity_counts['low']}**",
        "",
        "## Highest-count files",
        "",
        "| Count | File |",
        "|---:|---|",
    ]
    lines.extend(f"| {count} | `{file}` |" for file, count in file_counts.most_common(30))
    lines.extend(["", "## Finding kinds", "", "| Count | Kind |", "|---:|---|"])
    lines.extend(f"| {count} | `{kind}` |" for kind, count in kind_counts.most_common())
    lines.extend(["", "## High-confidence samples", "", "| File | Line | Kind | Text |", "|---|---:|---|---|"])
    for item in [finding for finding in findings if finding.severity == "high"][:200]:
        escaped = item.text.replace("|", "\\|").replace("\n", " ")
        lines.append(f"| `{item.file}` | {item.line} | `{item.kind}` | {escaped} |")
    lines.extend([
        "",
        "> This report is heuristic. Product names and exact technical identifiers are filtered, but review is still required before editing.",
        "",
    ])
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Find likely user-visible English hardcoded strings.")
    parser.add_argument("--tsv", default="artifacts/english-hardcoding-findings.tsv")
    parser.add_argument("--json", default="artifacts/english-hardcoding-findings.json")
    parser.add_argument("--markdown", default="artifacts/english-hardcoding-summary.md")
    args = parser.parse_args()

    files = iter_source_files()
    findings: list[Finding] = []
    for path in files:
        findings.extend(scan_source(path))
    severity_rank = {"high": 0, "medium": 1, "low": 2}
    findings.sort(key=lambda item: (severity_rank[item.severity], item.file, item.line, item.text))

    write_tsv(ROOT / args.tsv, findings)
    write_json(ROOT / args.json, findings)
    write_markdown(ROOT / args.markdown, findings, len(files))

    severity_counts = Counter(item.severity for item in findings)
    file_counts = Counter(item.file for item in findings)
    print(
        f"Scanned {len(files)} source files; found {len(findings)} candidate English hardcoded strings "
        f"(high={severity_counts['high']}, medium={severity_counts['medium']}, low={severity_counts['low']})."
    )
    print("Top files:")
    for file, count in file_counts.most_common(25):
        print(f"  {count:4d}  {file}")
    print("High-confidence samples:")
    for item in [finding for finding in findings if finding.severity == "high"][:150]:
        print(f"  {item.file}:{item.line} [{item.kind}] {item.text}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
