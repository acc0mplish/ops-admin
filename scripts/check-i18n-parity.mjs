#!/usr/bin/env node
// i18n 패리티 + 참조 무결 검증 (plan-bundle §7 C2, §4.1 G1)
//
// 검사 종류:
//   C2-1  dict 내 ko/en 키 집합 불일치
//   C2-2  ko/en {token} 플레이스홀더 집합 불일치 (string-aware 파서 — 값 리터럴 내부
//         'Datasource: {name}', '{"a":"b"}' 를 키로 오인하지 않는다, bundle M-3)
//   C2-3  dict 블록 내 중복 키 정의
//   C2-4a callsite→dict 역방향: 뷰·셸의 helper('key') 리터럴 키가 소속 dict에 존재
//   C2-4b dict→callsite: dict 키가 어떤 callsite에서도 참조되지 않음(죽은 키)
//   C2-5  dict별 파싱 키 수 출력 (검사 아님 — 기준선 출력)
//   X-APIERR  api-error-i18n 구조 예외 (bundle M-1): errorCodeKeys·fallback·legacy
//         패턴의 값 ⊆ common-i18n 키 집합 + errorCodeMessages ko/en 중첩 패리티
//
// 사용법:
//   node scripts/check-i18n-parity.mjs                      # 리포 전체 검사 (exit 0/1)
//   node scripts/check-i18n-parity.mjs --fixture <dir>      # known-bad fixture 검사 (결함 검출 시 비-0)
//   node scripts/check-i18n-parity.mjs --allowlist-sync     # en-extract/residual-allowlist.txt 재생성
//   node scripts/check-i18n-parity.mjs --allowlist-check    # C3 잔존 == allowlist 등재 수 검증
//   node scripts/check-i18n-parity.mjs --glossary [out]     # en-extract/glossary-ko-en.json 생성
//
// 참고: C2-4는 리터럴 1차 인자만 정적 검증한다. 동적 호출(helper(map[k] || 'fallback'))은
// fallback 리터럴까지 수집해 참조로 인정하되, 완전 동적 키는 정적 검증 불가로 집계만 한다.

import { readFileSync, writeFileSync, existsSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative, resolve } from 'node:path'

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(SCRIPT_DIR, '..')
const UTILS_DIR = join(ROOT, 'web/src/utils')
const CALLSITE_DIRS = [join(ROOT, 'web/src/views'), join(ROOT, 'web/src/layouts')]
const ALLOWLIST_PATH = join(ROOT, 'en-extract/residual-allowlist.txt')
const GLOSSARY_PATH = join(ROOT, 'en-extract/glossary-ko-en.json')
const HANGUL_RE = /[가-힣]/

// ---------------------------------------------------------------------------
// string-aware dict 파서
// ---------------------------------------------------------------------------

/** `const <name> = {` 블록의 시작 인덱스('{' 다음)를 찾는다. */
function findObjectBlockStart(source, name) {
  const re = new RegExp(`const\\s+${name}\\s*=\\s*\\{`)
  const m = re.exec(source)
  return m ? m.index + m[0].length : -1
}

function makeLineIndex(source) {
  const starts = [0]
  for (let i = 0; i < source.length; i++) {
    if (source.charCodeAt(i) === 10) starts.push(i + 1)
  }
  return (index) => {
    let lo = 0
    let hi = starts.length - 1
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1
      if (starts[mid] <= index) lo = mid
      else hi = mid - 1
    }
    return lo + 1
  }
}

function lineOf(source, index) {
  let line = 1
  for (let i = 0; i < index; i++) if (source.charCodeAt(i) === 10) line++
  return line
}

/**
 * 객체 리터럴 블록을 파싱해 엔트리를 수집한다.
 * 문자열 리터럴 내부의 콜론·중괄호·따옴표 이스케이프를 건너뛴다(M-3).
 * 반환: { entries: Map<key, {value, line, raw}>, end: 닫는 '}' 다음 인덱스, anomalies: string[] }
 * - 값이 문자열이면 value, 객체/배열 등이면 raw에 균형 잡힌 원문을 담는다.
 */
function parseObjectEntries(source, start) {
  const entries = new Map()
  const anomalies = []
  const duplicates = []
  let i = start
  let line = lineOf(source, start)
  let depth = 1

  const skipWs = () => {
    for (;;) {
      while (i < source.length && /\s/.test(source[i])) {
        if (source[i] === '\n') line++
        i++
      }
      if (source.startsWith('//', i)) {
        while (i < source.length && source[i] !== '\n') i++
        continue
      }
      if (source.startsWith('/*', i)) {
        const end = source.indexOf('*/', i + 2)
        if (end === -1) { i = source.length; return }
        for (let k = i; k < end; k++) if (source[k] === '\n') line++
        i = end + 2
        continue
      }
      return
    }
  }

  const readString = (quote) => {
    const startLine = line
    i++ // 여는 따옴표
    let out = ''
    while (i < source.length) {
      const ch = source[i]
      if (ch === '\\' && i + 1 < source.length) { out += ch + source[i + 1]; i += 2; continue }
      if (ch === quote) { i++; return out }
      if (ch === '\n') line++
      out += ch
      i++
    }
    anomalies.push(`종결되지 않은 문자열 리터럴(개행 ${startLine} 근처)`)
    return out
  }

  const readRawValue = () => {
    // 문자열이 아닌 값(중첩 객체·배열·식별자·숫자)을 균형 잡힌 원문으로 소비한다.
    const rawStart = i
    let localDepth = 0
    while (i < source.length) {
      const ch = source[i]
      if (ch === '\'' || ch === '"' || ch === '`') { readString(ch); continue }
      if (ch === '{' || ch === '[' || ch === '(') { localDepth++; i++; continue }
      if (ch === '}' && localDepth === 0) break
      if ((ch === '}' || ch === ']' || ch === ')')) { localDepth--; i++; continue }
      if (ch === ',' && localDepth === 0) break
      if (ch === '\n') line++
      i++
    }
    return source.slice(rawStart, i).trim()
  }

  while (i < source.length) {
    skipWs()
    if (i >= source.length) break
    const ch = source[i]
    if (ch === '}') { depth--; if (depth === 0) { i++; break } i++; continue }
    if (ch === '{') { depth++; i++; continue }
    // 키 읽기 (식별자 또는 따옴표 키)
    let key = null
    let keyLine = line
    if (/[A-Za-z0-9_$]/.test(ch)) {
      let j = i
      while (j < source.length && /[A-Za-z0-9_$]/.test(source[j])) j++
      key = source.slice(i, j)
      i = j
    } else if (ch === '\'' || ch === '"') {
      key = readString(ch)
    } else {
      i++ // 알 수 없는 문자 — 건너뜀
      continue
    }
    skipWs()
    if (source[i] !== ':') {
      anomalies.push(`키 '${key}' 뒤에 ':' 가 없음(${keyLine} 행)`)
      continue
    }
    i++
    skipWs()
    const vch = source[i]
    if (vch === '\'' || vch === '"' || vch === '`') {
      const value = readString(vch)
      if (entries.has(key)) duplicates.push({ key, line: keyLine })
      else entries.set(key, { value, line: keyLine, raw: null })
    } else {
      const raw = readRawValue()
      if (entries.has(key)) duplicates.push({ key, line: keyLine })
      else entries.set(key, { value: null, line: keyLine, raw })
    }
  }
  return { entries, end: i, anomalies, duplicates }
}

/** {{var}} 이중 토큰은 '{{var}}' 로, {name} 단일 토큰은 '{name}' 로 구별해 수집한다. */
function extractTokens(value) {
  const tokens = new Set()
  const stripped = value.replace(/\{\{\s*([^{}\s]+)\s*\}\}/g, (_, name) => {
    tokens.add(`{{${name}}}`)
    return ''
  })
  const re = /\{\s*([^{}\s]+)\s*\}/g
  let m
  while ((m = re.exec(stripped))) tokens.add(`{${m[1]}}`)
  return tokens
}

/** dict 파일에서 플랫 ko/en 블록을 파싱한다. */
function parseFlatDicts(source) {
  const result = {}
  for (const name of ['ko', 'en']) {
    const start = findObjectBlockStart(source, name)
    if (start === -1) continue
    const parsed = parseObjectEntries(source, start)
    result[name] = parsed
  }
  return result
}

// ---------------------------------------------------------------------------
// dict 카탈로그
// ---------------------------------------------------------------------------

// 리포 dict 파일 → 헬퍼명 (bundle §7 C2-4). fixture 모드는 export function에서 동적 유도.
const REPO_DICT_HELPERS = {
  'ai-model-i18n.js': ['amt'],
  'application-i18n.js': ['apt'],
  'asset-i18n.js': ['at'],
  'common-i18n.js': ['ct'],
  'dns-account-i18n.js': ['dat'],
  'domain-i18n.js': ['dt'],
  'english-hardcoding-i18n.js': ['uiT'],
  'finops-account-i18n.js': ['fat'],
  'finops-i18n.js': ['ft'],
  'integration-i18n.js': ['it'],
  'k8s-extra-i18n.js': ['kt'],
  'ldap-i18n.js': ['lt'],
  'monitor-i18n.js': ['mt'],
  'navigation-i18n.js': ['nt'],
  'notify-i18n.js': ['nt'],
  'ops-i18n.js': ['ot'],
  'system-i18n.js': ['st'],
  'topology-i18n.js': ['bt'],
  'i18n-runtime.js': ['t'],
  'i18n.js': ['t'],
}

function walkFiles(dir, exts, out = []) {
  if (!existsSync(dir)) return out
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    const st = statSync(full)
    if (st.isDirectory()) walkFiles(full, exts, out)
    else if (exts.some((e) => name.endsWith(e))) out.push(full)
  }
  return out
}

/** 파일 집합을 dict(헬퍼 유도 포함)와 callsite로 분류한다. fixture 모드용. */
function classifyFixtureDir(dir) {
  const files = readdirSync(dir).filter((f) => /\.(js|vue)$/.test(f)).map((f) => join(dir, f))
  const dicts = []
  const callsites = []
  for (const file of files) {
    const source = readFileSync(file, 'utf8')
    if (findObjectBlockStart(source, 'ko') !== -1) {
      const helperMatch = /export\s+function\s+([A-Za-z_$][\w$]*)\s*\(/.exec(source)
      const blocks = parseFlatDicts(source)
      const empty = { entries: new Map(), anomalies: [], duplicates: [] }
      dicts.push({
        file,
        base: relative(dir, file).replaceAll('\\', '/'),
        helpers: helperMatch ? [helperMatch[1]] : [],
        ko: blocks.ko || empty,
        en: blocks.en || null,
        koOnly: !blocks.en,
      })
    } else {
      callsites.push({ file, source })
    }
  }
  return { dicts, callsites }
}

function loadRepoDicts() {
  const dicts = []
  const knownFiles = new Set(Object.keys(REPO_DICT_HELPERS))
  let apiError = null
  for (const file of walkFiles(UTILS_DIR, ['.js'])) {
    const base = relative(UTILS_DIR, file)
    if (!knownFiles.has(base)) {
      const probe = readFileSync(file, 'utf8')
      if (findObjectBlockStart(probe, 'ko') !== -1) {
        findings.push(['WARN', `[WARN] 미등록 dict 후보 발견 — helper 매핑 없어 검사 제외: web/src/utils/${base}`])
      }
      continue
    }
    const source = readFileSync(file, 'utf8')
    if (base === 'api-error-i18n.js') {
      apiError = { file, base, source }
      continue
    }
    const blocks = parseFlatDicts(source)
    dicts.push({
      file, base, helpers: REPO_DICT_HELPERS[base],
      ko: blocks.ko || { entries: new Map(), anomalies: [], duplicates: [] },
      en: blocks.en || null,
      koOnly: !blocks.en,
    })
  }
  return { dicts, apiError }
}

// ---------------------------------------------------------------------------
// findings 수집
// ---------------------------------------------------------------------------

const findings = []
function finding(tag, message) {
  findings.push([tag, `[${tag}] ${message}`])
}

function checkDictParity(dict, koFallbackKeys) {
  const label = `web/src/utils/${dict.base}`
  for (const anomaly of dict.ko.anomalies) finding('C2-PARSE', `${label} ko 블록: ${anomaly}`)
  if (dict.ko.duplicates.length) {
    for (const dup of dict.ko.duplicates) finding('C2-3', `${label} ko 블록 중복 키 '${dup.key}' (${dup.line} 행 재선언)`)
  }
  if (dict.koOnly) return
  if (!dict.en) { finding('C2-1', `${label} en 블록 없음`); return }
  for (const anomaly of dict.en.anomalies) finding('C2-PARSE', `${label} en 블록: ${anomaly}`)
  for (const dup of dict.en.duplicates) finding('C2-3', `${label} en 블록 중복 키 '${dup.key}' (${dup.line} 행 재선언)`)

  const koKeys = new Set(dict.ko.entries.keys())
  const enKeys = new Set(dict.en.entries.keys())
  for (const k of koKeys) if (!enKeys.has(k)) {
    finding('C2-1', `${label} en 키 누락: '${k}' (ko ${dict.ko.entries.get(k).line} 행)`)
  }
  // i18n-runtime은 ko 미보유 키를 i18n.js(KO canonical)로 위임한다(t(): ko[key] || koText(key)).
  // 따라서 en-only 키의 무결 기준은 'runtime.ko ∪ i18n.js.ko' 집합이다.
  for (const k of enKeys) if (!koKeys.has(k) && !(koFallbackKeys && koFallbackKeys.has(k))) {
    finding('C2-1', `${label} ko 키 누락: '${k}' (en ${dict.en.entries.get(k).line} 행)`)
  }
  for (const k of koKeys) {
    if (!enKeys.has(k)) continue
    const koValue = dict.ko.entries.get(k).value ?? ''
    const enValue = dict.en.entries.get(k).value ?? ''
    const koTokens = extractTokens(koValue)
    const enTokens = extractTokens(enValue)
    const missingEn = [...koTokens].filter((t) => !enTokens.has(t))
    const missingKo = [...enTokens].filter((t) => !koTokens.has(t))
    for (const t of missingEn) finding('C2-2', `${label} '${k}' 플레이스홀더 불일치 — ko에만 있음 ${t}`)
    for (const t of missingKo) finding('C2-2', `${label} '${k}' 플레이스홀더 불일치 — en에만 있음 ${t}`)
  }
}

// ---------------------------------------------------------------------------
// callsite 스캔
// ---------------------------------------------------------------------------

const HELPER_CALL_RE = /(?<![\w.$])([A-Za-z_$][\w$]*)\(\s*'((?:[^'\\]|\\.)*)'/g
// fallback 리터럴 수집 — helper(map[k] || 'key') 형태. `(`와 `||` 사이에 문자열 리터럴이
// 있으면 그 fallback은 params 객체의 값이지 dict 키가 아니므로 제외한다([' 제외).
const HELPER_FALLBACK_RE = /(?<![\w.$])([A-Za-z_$][\w$]*)\(([^'()]*?)\|\|\s*'((?:[^'\\]|\\.)*)'/g

function scanCallsites(callsiteFiles, dicts, dynamicStats) {
  const helperDicts = new Map() // helper → [dict, ...]
  for (const d of dicts) for (const h of d.helpers) {
    if (!helperDicts.has(h)) helperDicts.set(h, [])
    helperDicts.get(h).push(d)
  }
  const helperNames = [...helperDicts.keys()]
  const refs = [] // { file, line, helper, key }
  for (const { file, source } of callsiteFiles) {
    // 파일 내 import로 헬퍼 → dict 소속 확정 (nt 동명 충돌 해소)
    const importMap = new Map() // local name → helper
    const importRe = /import\s*\{([^}]+)\}\s*from\s*'([^']+)'(?:;|$)/gm
    let m
    while ((m = importRe.exec(source))) {
      const sourceBase = m[2].split('/').pop().replace(/\.js$/, '')
      const target = dicts.find((d) => d.base.replace(/\.js$/, '') === sourceBase)
      if (!target) continue
      for (const rawName of m[1].split(',')) {
        const name = rawName.trim()
        if (!name) continue
        const asMatch = /^([\w$]+)\s+as\s+([\w$]+)$/.exec(name)
        const helper = asMatch ? asMatch[1] : name
        const local = asMatch ? asMatch[2] : name
        importMap.set(local, helper)
      }
    }
    const lineAt = makeLineIndex(source)
    const collect = (re, keyGroup) => {
      re.lastIndex = 0
      let hit
      while ((hit = re.exec(source))) {
        const local = hit[1]
        const key = hit[keyGroup].replace(/\\'/g, "'")
        const helper = importMap.get(local) ?? local
        if (!helperNames.includes(helper)) continue
        refs.push({ file, line: lineAt(hit.index), helper, key })
      }
    }
    collect(HELPER_CALL_RE, 2)
    collect(HELPER_FALLBACK_RE, 3)
    // 정적 검증 불가 동적 호출 집계
    const dynRe = /(?<![\w.$])([A-Za-z_$][\w$]*)\(\s*[^'\s)]/g
    while ((m = dynRe.exec(source))) {
      const local = m[1]
      const helper = importMap.get(local) ?? local
      if (!helperDicts.has(helper)) continue
      dynamicStats.push({ file: relative(ROOT, file), line: lineAt(m.index), helper })
    }
  }
  return { refs, helperDicts }
}

function resolveRefDicts(ref, helperDicts) {
  const candidates = helperDicts.get(ref.helper) || []
  return candidates.filter((d) => {
    if (d.koOnly) return d.ko.entries.has(ref.key)
    return d.ko.entries.has(ref.key) || (d.en && d.en.entries.has(ref.key))
  })
}

function checkCallsiteIntegrity(dicts, refs, helperDicts, options) {
  // C2-4a: callsite 키가 소속 dict에 존재해야 한다
  for (const ref of refs) {
    const candidates = helperDicts.get(ref.helper) || []
    if (!candidates.length) continue
    const resolved = resolveRefDicts(ref, helperDicts)
    if (resolved.length === 0) {
      const rel = relative(ROOT, ref.file)
      finding('C2-4a', `${rel}:${ref.line} helper '${ref.helper}(' 키 미등록: '${ref.key}'`)
    }
  }
  // C2-4b: dict 키가 최소 1개 callsite에서 참조되어야 한다 (판단 E — fallback 은폐 차단)
  const referenced = new Map() // dict.base → Set<key>
  for (const d of dicts) referenced.set(d.base, new Set())
  for (const ref of refs) {
    for (const d of resolveRefDicts(ref, helperDicts)) referenced.get(d.base).add(ref.key)
  }
  // api-error errorCodeKeys·legacy 패턴의 값도 common-i18n 키 참조로 인정한다
  for (const key of options.extraKeys || []) {
    if (referenced.has('common-i18n.js')) referenced.get('common-i18n.js').add(key)
  }
  let unreferencedTotal = 0
  for (const d of dicts) {
    const refKeys = referenced.get(d.base)
    const dead = [...d.ko.entries.keys()].filter((k) => !refKeys.has(k))
    if (dead.length) {
      unreferencedTotal += dead.length
      if (options.failUnreferenced) {
        const label = `web/src/utils/${d.base}`
        const shown = dead.slice(0, 10).map((k) => `'${k}'`).join(', ')
        const more = dead.length > 10 ? ` 외 ${dead.length - 10}건` : ''
        finding('C2-4b', `${label} callsite 미참조 키 ${dead.length}건: ${shown}${more}`)
      }
    }
  }
  return unreferencedTotal
}

// ---------------------------------------------------------------------------
// api-error-i18n 구조 예외 (bundle M-1)
// ---------------------------------------------------------------------------

function checkApiError(apiError, commonDict) {
  if (!apiError || !commonDict) return
  const label = `web/src/utils/${apiError.base}`
  const commonKeys = new Set(commonDict.ko.entries.keys())
  if (!commonDict.koOnly && commonDict.en) for (const k of commonDict.en.entries.keys()) commonKeys.add(k)

  const keysStart = findObjectBlockStart(apiError.source, 'errorCodeKeys')
  if (keysStart !== -1) {
    const { entries } = parseObjectEntries(apiError.source, keysStart)
    for (const [code, { value, line }] of entries) {
      if (!commonKeys.has(value)) {
        finding('X-APIERR', `${label}:${line} errorCodeKeys['${code}'] → common-i18n 미등록 키 '${value}'`)
      }
    }
  }
  const msgsStart = findObjectBlockStart(apiError.source, 'errorCodeMessages')
  if (msgsStart !== -1) {
    const { entries } = parseObjectEntries(apiError.source, msgsStart)
    for (const [code, { raw, line }] of entries) {
      if (!raw) { finding('X-APIERR', `${label}:${line} errorCodeMessages['${code}'] 값이 중첩 객체가 아님`); continue }
      const ko = /(?:^|[{,\s])ko\s*:\s*'((?:[^'\\]|\\.)*)'/.exec(raw)
      const en = /(?:^|[{,\s])en\s*:\s*'((?:[^'\\]|\\.)*)'/.exec(raw)
      if (!ko || !en) { finding('X-APIERR', `${label}:${line} errorCodeMessages['${code}'] ko/en 쌍 누락`); continue }
      const koTokens = extractTokens(ko[1])
      const enTokens = extractTokens(en[1])
      for (const t of koTokens) if (!enTokens.has(t)) finding('C2-2', `${label} errorCodeMessages['${code}'] 플레이스홀더 불일치 — ko에만 있음 ${t}`)
      for (const t of enTokens) if (!koTokens.has(t)) finding('C2-2', `${label} errorCodeMessages['${code}'] 플레이스홀더 불일치 — en에만 있음 ${t}`)
    }
  }
  // 참조 무결 보강: fallback 기본키 + legacy 영문 패턴 매핑 값도 common-i18n 키
  const extraRefs = []
  const fallback = /fallbackKey\s*=\s*'([^']+)'/.exec(apiError.source)
  if (fallback) extraRefs.push(fallback[1])
  const legacyBlock = /const\s+legacyEnglishPatterns\s*=\s*\[/
  if (legacyBlock.test(apiError.source)) {
    const re = /,\s*'([A-Za-z][\w]*)'\]/g
    let m
    while ((m = re.exec(apiError.source))) extraRefs.push(m[1])
  }
  for (const key of extraRefs) {
    if (!commonKeys.has(key)) finding('X-APIERR', `${label} common-i18n 미등록 키 참조: '${key}'`)
  }
  return extraRefs
}

// ---------------------------------------------------------------------------
// C3 잔존 스캔 (allowlist)
// ---------------------------------------------------------------------------

/** bundle §7 C3 — 콘텐츠 컬럼 고정 앵커형 필터(C-1 수정판). */
function isCommentAnchored(content) {
  return /^\s*(\/\/|\/\*|\*|<!--)/.test(content)
}

function scanResiduals() {
  const rows = []
  for (const dir of CALLSITE_DIRS) {
    for (const file of walkFiles(dir, ['.vue'])) {
      const rel = relative(ROOT, file).replaceAll('\\', '/')
      const source = readFileSync(file, 'utf8')
      const lines = source.split('\n')
      lines.forEach((content, idx) => {
        if (!HANGUL_RE.test(content)) return
        if (isCommentAnchored(content)) return
        rows.push({ file: rel, line: idx + 1, content: content.trim() })
      })
    }
  }
  return rows
}

function classifyResidual(content) {
  if (/===|!==|switch\s*\(|case\s+|value\s*:/.test(content)) return '비교·송신값 후보(§3-1 검증 필요)'
  if (/title\s*:\s*'|content\s*:\s*'|summary\s*:\s*'|detail\s*:\s*'/.test(content)) return 'seed·데이터 구조 표시값(판단 D)'
  if (/https?:\/\//.test(content)) return '표시 문자열 내 URL(E규칙 — URL 무손상)'
  return '표시 문자열(마이그레이션 대상)'
}

function syncAllowlist() {
  const rows = scanResiduals()
  const header = [
    '# en-extract 잔존 한글 라인 등재부 (C3 — bundle §7)',
    '# 필터: 한글 포함 && 콘텐츠 열이 //, /*, *, <!-- 로 시작하지 않는 라인 (C-1 수정 앵커형)',
    '# 형식: <repo-relative file>:<line>: <사유>',
    '# 유지보수 계약: 각 레인 페이즈가 자기 파일의 해소 라인을 삭제하고, 페이즈 종료 시',
    '#   `node scripts/check-i18n-parity.mjs --allowlist-check` 로 등재 수 == 스캔 수 일치를 검증한다.',
    `# 생성: ${new Date().toISOString()} / 총 ${rows.length}건`,
    '',
  ]
  const body = rows.map((r) => `${r.file}:${r.line}: ${classifyResidual(r.content)}`)
  writeFileSync(ALLOWLIST_PATH, [...header, ...body, ''].join('\n'))
  console.log(`allowlist 재생성: ${ALLOWLIST_PATH} (${rows.length}건)`)
}

function checkAllowlist() {
  const rows = scanResiduals()
  if (!existsSync(ALLOWLIST_PATH)) {
    finding('C3', `allowlist 부재: ${relative(ROOT, ALLOWLIST_PATH)} — --allowlist-sync 로 생성`)
    return
  }
  const listed = readFileSync(ALLOWLIST_PATH, 'utf8')
    .split('\n')
    .filter((l) => l.trim() && !l.startsWith('#'))
  const listedSet = new Set(listed.map((l) => l.replace(/: [^:]*$/, '').trim()))
  const scanSet = new Set(rows.map((r) => `${r.file}:${r.line}`))
  let mismatch = 0
  for (const key of scanSet) if (!listedSet.has(key)) {
    if (mismatch < 15) finding('C3', `스캔 잔존인데 미등재: ${key}`)
    mismatch++
  }
  for (const key of listedSet) if (!scanSet.has(key)) {
    if (mismatch < 15) finding('C3', `등재됐으나 이미 해소(삭제 필요): ${key}`)
    mismatch++
  }
  if (rows.length !== listed.length || mismatch > 0) {
    finding('C3', `잔존 스캔 ${rows.length}건 != allowlist 등재 ${listed.length}건`)
  }
}

// ---------------------------------------------------------------------------
// glossary
// ---------------------------------------------------------------------------

function writeGlossary(outPath) {
  const { dicts } = loadRepoDicts()
  const entries = new Map()
  let totalPairs = 0
  for (const d of dicts) {
    if (d.koOnly || !d.en) continue
    for (const [key, koEntry] of d.ko.entries) {
      const enEntry = d.en.entries.get(key)
      if (!enEntry || koEntry.value == null || enEntry.value == null) continue
      if (!entries.has(koEntry.value)) entries.set(koEntry.value, [])
      entries.get(koEntry.value).push({ en: enEntry.value, dict: d.base, key })
      totalPairs++
    }
  }
  const sorted = [...entries.entries()].sort((a, b) => (a[0] < b[0] ? -1 : 1))
  const out = {
    description: '기존 19 dict에서 추출한 ko→en 대응어 집 (bundle M-4 — 신규 번역 전 먼저 조회해 기존 대응어 재사용)',
    generatedAt: new Date().toISOString(),
    sourceDicts: dicts.filter((d) => !d.koOnly).map((d) => d.base),
    totalPairs,
    entries: Object.fromEntries(sorted.map(([ko, list]) => [ko, { en: list.map((x) => x.en), refs: list }])),
  }
  writeFileSync(outPath, JSON.stringify(out, null, 2) + '\n')
  console.log(`glossary 생성: ${outPath} — ko 표현 ${sorted.length}종 / 쌍 ${totalPairs}건`)
}

// ---------------------------------------------------------------------------
// 모드 진입
// ---------------------------------------------------------------------------

function runChecks({ dicts, apiError, callsiteFiles, failUnreferenced, extraReferences }) {
  const i18nJsKo = dicts.find((d) => d.base === 'i18n.js')
  const koFallbackKeys = i18nJsKo ? new Set(i18nJsKo.ko.entries.keys()) : null
  for (const d of dicts) checkDictParity(d, d.base === 'i18n-runtime.js' ? koFallbackKeys : null)
  const dynamicStats = []
  const { refs, helperDicts } = scanCallsites(callsiteFiles, dicts, dynamicStats)
  let apiExtraRefs = []
  if (apiError) {
    const common = dicts.find((d) => d.base === 'common-i18n.js')
    apiExtraRefs = checkApiError(apiError, common) || []
  }
  const unreferencedTotal = checkCallsiteIntegrity(dicts, refs, helperDicts, {
    failUnreferenced,
    extraKeys: apiExtraRefs,
  })
  // C2-5 기준선 출력
  console.log('== C2-5 dict 키 수 기준선 ==')
  for (const d of dicts) {
    const koCount = d.ko.entries.size
    const enCount = d.en ? d.en.entries.size : '-'
    const helpers = d.helpers.join('/')
    console.log(`  web/src/utils/${d.base.padEnd(30)} helper=${helpers.padEnd(4)} ko=${String(koCount).padEnd(5)} en=${enCount}`)
  }
  console.log(`  callsite 리터럴 참조 ${refs.length}건 / 동적 호출(정적 검증 제외) ${dynamicStats.length}건 / callsite 미참조 키 ${unreferencedTotal}건${failUnreferenced ? '(FAIL)' : '(INFO)'}`)
}

function main() {
  const args = process.argv.slice(2)
  const fixtureIdx = args.indexOf('--fixture')
  if (fixtureIdx !== -1) {
    const dir = resolve(args[fixtureIdx + 1])
    const { dicts, callsites } = classifyFixtureDir(dir)
    if (!dicts.length) { console.error(`fixture에 dict(const ko = {) 파일이 없습니다: ${dir}`); process.exit(2) }
    runChecks({ dicts, apiError: null, callsiteFiles: callsites, failUnreferenced: true })
    if (findings.length) {
      for (const [, msg] of findings) console.log(msg)
      console.log(`FIXTURE-RESULT: FAIL (${findings.length}건 검출 — ${[...new Set(findings.map(([t]) => t))].join(',')})`)
      process.exit(1)
    }
    console.log('FIXTURE-RESULT: PASS (결함 0 — 네거티브 컨트롤이 검출하지 못함, 이 fixture는 결함이어야 함)')
    process.exit(1)
  }
  if (args.includes('--allowlist-sync')) { syncAllowlist(); process.exit(0) }
  if (args.includes('--allowlist-check')) {
    checkAllowlist()
    if (findings.length) {
      for (const [, msg] of findings) console.log(msg)
      process.exit(1)
    }
    console.log('C3 잔존 스캔 == allowlist 등재 수 일치')
    process.exit(0)
  }
  if (args.includes('--glossary')) {
    const outIdx = args.indexOf('--glossary')
    const out = args[outIdx + 1] && !args[outIdx + 1].startsWith('--') ? resolve(args[outIdx + 1]) : GLOSSARY_PATH
    writeGlossary(out)
    process.exit(0)
  }

  // 기본: 리포 전체 검사
  const { dicts, apiError } = loadRepoDicts()
  const callsiteFiles = CALLSITE_DIRS.flatMap((dir) =>
    walkFiles(dir, ['.vue', '.js']).map((file) => ({ file, source: readFileSync(file, 'utf8') })))
  // 기존 정상 뷰의 죽은 키까지 즉시 FAIL 하지 않도록(마이그레이션 중 안정성) 기본은 INFO,
  // --strict 로 레인 게이트에서 FAIL 로 승격한다. (fixture 모드는 항상 FAIL)
  const failUnreferenced = args.includes('--strict')
  runChecks({ dicts, apiError, callsiteFiles, failUnreferenced, extraReferences: null })
  checkAllowlist()
  if (findings.length) {
    const tagIdx = args.indexOf('--tag')
    const tagFilter = tagIdx !== -1 ? args[tagIdx + 1] : null
    const byTag = {}
    for (const [tag] of findings) byTag[tag] = (byTag[tag] || 0) + 1
    const visible = tagFilter ? findings.filter(([tag]) => tag === tagFilter) : findings
    const limit = 200
    for (const [, msg] of visible.slice(0, limit)) console.log(msg)
    if (visible.length > limit) console.log(`... 외 ${visible.length - limit}건`)
    console.log(`RESULT: FAIL — ${findings.length}건 (${Object.entries(byTag).map(([t, n]) => `${t}:${n}`).join(' ')})`)
    process.exit(1)
  }
  console.log('RESULT: PASS — C2 패리티·참조 무결·C3 allowlist 일치')
  process.exit(0)
}

main()
