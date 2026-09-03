#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def replace_exact(path: Path, old: str, new: str, expected: int = 1) -> None:
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count != expected:
        raise SystemExit(f"{path}: expected {expected} occurrence(s) of {old!r}, found {count}")
    path.write_text(text.replace(old, new), encoding="utf-8")


def patch_command_center() -> None:
    path = ROOT / "web/src/views/monitor/MonitorCommandCenter.vue"
    replace_exact(
        path,
        "import { queryMonitorCommandCenter } from '../../api/monitor'\n",
        "import { queryMonitorCommandCenter } from '../../api/monitor'\nimport { currentLocale } from '../../utils/i18n-runtime'\nimport { mt } from '../../utils/monitor-i18n'\n",
    )
    replace_exact(path, "toLocaleTimeString('zh-CN',", "toLocaleTimeString(currentLocale.value,", 1)
    replace_exact(path, "toLocaleDateString('zh-CN',", "toLocaleDateString(currentLocale.value,", 1)
    replacements = {
        '<small>OPS ADMIN INTELLIGENT OPERATIONS CENTER</small>': "<small>{{ mt('commandCenterEyebrow') }}</small>",
        '<small>RESOURCE SITUATION</small>': "<small>{{ mt('resourceSituation') }}</small>",
        '<span>PHYSICAL HOST</span>': "<span>{{ mt('physicalHost') }}</span>",
        '<span>MONITORING</span>': "<span>{{ mt('monitoring') }}</span>",
        '<span>ACTIVE ALERT</span>': "<span>{{ mt('activeAlert') }}</span>",
        '<span>AUTH RATE</span>': "<span>{{ mt('authRate') }}</span>",
        '<em>TOP 5</em>': "<em>{{ mt('topFive') }}</em>",
        '<em>TOP {{ data.resourceComposition?.length || 0 }}</em>': "<em>{{ mt('topCount', { count: data.resourceComposition?.length || 0 }) }}</em>",
    }
    for old, new in replacements.items():
        replace_exact(path, old, new)


def patch_navigation_count() -> None:
    path = ROOT / "web/src/views/integration/IntegrationNavigation.vue"
    replace_exact(
        path,
        '<span class="navigation-count"><strong>{{ navigations.length }}</strong> {{ nt(\'navigationCount\', { count: navigations.length }).replace(String(navigations.length), \'\').trim() }}</span>',
        '<span class="navigation-count">{{ nt(\'navigationCount\', { count: navigations.length }) }}</span>',
    )


if __name__ == "__main__":
    patch_command_center()
    patch_navigation_count()
    print("Applied frontend English hardcoding remediation batch 1.")
