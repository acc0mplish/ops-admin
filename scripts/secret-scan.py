#!/usr/bin/env python3
"""Tree-level secret scanner for CI (V2 Phase -1 Task 6, plan D-2).

High-precision rules only: every rule must fire on a known incident class and
must not fire on the current tree (measured 2026-09-06, plan §0). New rules
are added together with the allowlist entry that documents their known
benign occurrence — an allowlist entry is "value ∥ path glob": the value is
tolerated only at that path and anywhere else it is a finding.

The full git history is deliberately NOT scanned (plan A4): backend/config.yaml
and deploy/config.yaml exist in history before they were untracked, so a
history scan is permanently red. The guard below asserts instead that they
stay untracked and remain git-ignored, and scans the current tree only.

Usage: python3 scripts/secret-scan.py [target-dir=.]
"""

import fnmatch
import os
import re
import subprocess
import sys

# H2 self-exclusion: the scanner source itself carries rule/allowlist literals
# by definition, so its path (relative to the scan root, hence also true for a
# scratch copy of the whole repo) is exempt from value scanning. This path
# scoping is what lets the workflow canary keep the same literals inside
# .github/workflows/* while the planted scratch file still fires.
SELF_RELATIVE_PATH = "scripts/secret-scan.py"

# Skipped directory names (relative to any level).
SKIPPED_DIRS = {".git", "node_modules", "dist", "uploads", ".tmp"}

# Rules: (name, compiled pattern, value extractor). The extractor returns the
# credential substring so the allowlist can key on the value itself.
RULES = [
    (
        "aws-access-key-id",
        re.compile(r"AKIA[0-9A-Z]{16}"),
        lambda m: m.group(0),
    ),
    (
        "aws-secret-assignment",
        re.compile(
            r"aws.{0,30}(?:secret|sk).{0,20}['\"]([A-Za-z0-9/+=]{40})['\"]",
            re.IGNORECASE,
        ),
        lambda m: m.group(1),
    ),
    (
        "private-key-header",
        re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
        lambda m: m.group(0),
    ),
    (
        "github-token",
        re.compile(r"(?:ghp_|gho_|ghs_)[A-Za-z0-9]{36}"),
        lambda m: m.group(0),
    ),
    (
        "slack-token",
        re.compile(r"xox[baprs]-"),
        lambda m: m.group(0),
    ),
    (
        "openai-style-key",
        re.compile(r"sk-[A-Za-z0-9]{20,}"),
        lambda m: m.group(0),
    ),
    # H3 keyword+entropy: covers the 89b81b0 incident class (credential-key
    # with a 44-char pure [A-Za-z0-9+/=-] value that no prefix rule matches).
    # The hyphen/underscore in the value class are required for that class;
    # the repo-wide false-positive census found exactly one hit and it is
    # allowlisted below (plan §0).
    (
        "keyword-entropy",
        re.compile(
            r"(?i)(?:password|secret|token|credential[-_]?key)"
            r"[\"']?\s*[:=]\s*[\"']([A-Za-z0-9+/=_-]{32,})[\"']"
        ),
        lambda m: m.group(1),
    ),
]

# value ∥ path glob (relative to the scan root, forward slashes).
ALLOWLIST = [
    # AWS documentation example pair — kept as the workflow canary literals.
    ("AKIAIOSFODNN7EXAMPLE", ".github/workflows/*"),
    ("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ".github/workflows/*"),
    # UI placeholder in the SSL upload dialog (no key body follows).
    ("-----BEGIN PRIVATE KEY-----", "web/src/views/domains/SSLCertificates.vue"),
    # Documented placeholder of the example config template.
    ("REPLACE_WITH_RANDOM_32BYTE_BASE64_KEY", "backend/config.example.yaml"),
    # The same four benign literals are quoted inside the V2 planning docs
    # (task6-plan.md spells out the canary values it will plant). Scoped to
    # the planning tree only — the same values anywhere else still fire.
    ("AKIAIOSFODNN7EXAMPLE", "v2-phase1/*"),
    ("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "v2-phase1/*"),
    ("-----BEGIN PRIVATE KEY-----", "v2-phase1/*"),
    ("REPLACE_WITH_RANDOM_32BYTE_BASE64_KEY", "v2-phase1/*"),
]

# Untracked-and-ignored guard for the previously leaked config files (plan A4).
HISTORY_GUARD_FILES = ("backend/config.yaml", "deploy/config.yaml")


def iter_files(root):
    ignored = ignored_paths(root)
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIPPED_DIRS)
        for name in sorted(filenames):
            path = os.path.join(dirpath, name)
            if os.path.islink(path):
                continue
            rel = os.path.relpath(path, root).replace(os.sep, "/")
            if rel == SELF_RELATIVE_PATH:
                continue
            if rel in ignored:
                # Git-ignored files never enter the repository (config.yaml is
                # the designed local-secret file), so they are out of scope.
                continue
            yield rel, path


def ignored_paths(root):
    """Relative paths of git-ignored files; empty when not a git repo."""
    probe = run_git(root, ["rev-parse", "--show-toplevel"])
    if probe.returncode != 0 or not os.path.exists(os.path.join(root, ".git")):
        return set()
    listing = run_git(root, ["ls-files", "--others", "--ignored", "--exclude-standard", "-z"])
    if listing.returncode != 0:
        print("NOTE: could not list ignored files (%s) — scanning everything."
              % listing.stderr.strip())
        return set()
    return {p for p in listing.stdout.split("\0") if p}


def redact(value):
    return value[:6] + "…(len=%d)" % len(value)


def scan_tree(root):
    findings = []
    scanned = 0
    for rel, path in iter_files(root):
        try:
            with open(path, "rb") as handle:
                data = handle.read()
        except OSError as exc:
            findings.append((rel, 0, "unreadable", str(exc)))
            continue
        if b"\x00" in data:
            continue  # binary (sqlite fixtures and friends)
        try:
            text = data.decode("utf-8")
        except UnicodeDecodeError:
            continue  # undecodable binary
        scanned += 1
        for lineno, line in enumerate(text.splitlines(), 1):
            for name, pattern, extract in RULES:
                for match in pattern.finditer(line):
                    value = extract(match)
                    if is_allowlisted(value, rel):
                        continue
                    findings.append((rel, lineno, name, redact(value)))
    return findings, scanned


def is_allowlisted(value, rel):
    for allowed_value, allowed_glob in ALLOWLIST:
        if value == allowed_value and fnmatch.fnmatch(rel, allowed_glob):
            return True
    return False


def run_git(root, args):
    return subprocess.run(
        ["git", "-C", root, *args],
        capture_output=True,
        text=True,
        check=False,
    )


def history_guard(root):
    """Assert the untracked config files stay untracked and git-ignored."""
    probe = run_git(root, ["rev-parse", "--show-toplevel"])
    if probe.returncode != 0 or not os.path.exists(os.path.join(root, ".git")):
        print("NOTE: no git repository at scan root — history guard skipped.")
        return
    tracked = run_git(root, ["ls-files"])
    if tracked.returncode != 0:
        raise SystemExit("secret-scan: git ls-files failed: %s" % tracked.stderr)
    leaked = [p for p in HISTORY_GUARD_FILES if p in tracked.stdout.splitlines()]
    if leaked:
        for path in leaked:
            print("SECRET-GUARD: tracked config file must stay untracked: %s" % path)
        raise SystemExit(1)
    with open(os.path.join(root, ".gitignore"), encoding="utf-8") as handle:
        ignored = {line.strip() for line in handle}
    missing = [p for p in HISTORY_GUARD_FILES if p not in ignored]
    if missing:
        for path in missing:
            print("SECRET-GUARD: .gitignore lost its entry for %s" % path)
        raise SystemExit(1)
    print("PASS-HISTORY-GUARD: %s untracked and git-ignored." % ", ".join(HISTORY_GUARD_FILES))


def main(argv):
    root = os.path.abspath(argv[1]) if len(argv) > 1 else os.getcwd()
    if not os.path.isdir(root):
        print("secret-scan: target is not a directory: %s" % root)
        return 2
    history_guard(root)
    findings, scanned = scan_tree(root)
    if findings:
        for rel, lineno, rule, value in findings:
            print("SECRET: %s:%d: [%s] value=%s" % (rel, lineno, rule, value))
        print(
            "FAIL: %d secret finding(s) across %d file(s). "
            "Add an allowlist entry (value ∥ path) only for a documented "
            "benign occurrence." % (len(findings), len({f[0] for f in findings}))
        )
        return 1
    print("PASS: secret-scan clean (%d files, %d rules)." % (scanned, len(RULES)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
