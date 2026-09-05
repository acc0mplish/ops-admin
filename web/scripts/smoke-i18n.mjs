#!/usr/bin/env node
// i18n 스모크 — C4(EN 잔존 한글) / C5(KO 무회귀 baseline 비교) / C6(ElementPlus 양방향)
// 배치 근거: bundle §4.1-4 (H-5) — node 모듈 해석이 web/node_modules 를 향하도록 web/ 산하에 둔다.
//
// 사용법:
//   node scripts/smoke-i18n.mjs --locale=en-US [--dialogs] [옵션]
//   node scripts/smoke-i18n.mjs --locale=ko-KR           # C5: en-extract/ko-baseline.json 이 있을 때 비교
//
// 옵션:
//   --base-url=URL     기본 http://localhost:8080 (vite dev) — env SMOKE_BASE_URL
//   --token=JWT        localStorage 에 주입 (env SMOKE_TOKEN) — 미인증 축소 실행도 가능
//   --user= --pass=    로그인 폼으로 실제 로그인 (env SMOKE_USERNAME / SMOKE_PASSWORD)
//   --dialogs          ElMessageBox 샘플링(C4 사해 셀렉터 대체 — 삭제 확인 다이얼로그 열기)
//   --routes=/a,/b     기본 대표 라우트 대체
//   --out=DIR          스크린샷 저장소 (기본 <repo>/en-extract/shots)
//
// exit code: 0 통과(또는 문서화된 축소 통과) / 1 검증 FAIL(EN 한글 잔존, KO baseline 불일치)
//            2 환경 미비(playwright·브라우저 없음) / 3 dev server 접속 불가

import { createRequire } from 'node:module'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(SCRIPT_DIR, '..', '..')
const args = process.argv.slice(2)

const argOf = (name, fallback = null) => {
  const hit = args.find((a) => a.startsWith(`--${name}=`))
  return hit ? hit.slice(name.length + 3) : fallback
}
const hasFlag = (name) => args.includes(`--${name}`)

const locale = argOf('locale', 'ko-KR')
if (!['ko-KR', 'en-US'].includes(locale)) {
  console.error(`--locale 은 en-US|ko-KR 만 허용: 받은 값 '${locale}'`)
  process.exit(2)
}
const baseUrl = argOf('base-url', process.env.SMOKE_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')
const token = argOf('token', process.env.SMOKE_TOKEN || null)
const user = argOf('user', process.env.SMOKE_USERNAME || null)
const pass = argOf('pass', process.env.SMOKE_PASSWORD || null)
const dialogs = hasFlag('dialogs')
const captureBaseline = hasFlag('capture-baseline')
const outDir = resolve(argOf('out', join(ROOT, 'en-extract/shots')))

// C4 대표 라우트(레인별 1+) + 로그인. C5 비교 대상과 동일 집합을 쓴다.
const DEFAULT_ROUTES = [
  '/login',
  '/dashboard',
  '/assets/server/hosts',
  '/assets/databases',
  '/containers/k8s/clusters',
  '/containers/services',
  '/monitor/dashboards',
  '/monitor/alert-rules',
  '/ops/scripts/library',
  '/notify/templates',
  '/domains/public',
  '/integration/finops/dashboard',
  '/system/admin',
]
const routes = argOf('routes') ? argOf('routes').split(',').map((r) => r.trim()).filter(Boolean) : DEFAULT_ROUTES

// C4 검증 셀렉터(bundle §7 C4 — 사해 셀렉터 .el-message·.el-dialog__title 제외,
// td는 백엔드 한국어 데이터 오염 가능성으로 WARN 강등, ElMessageBox는 --dialogs 샘플링으로 이동)
const FAIL_SELECTOR = [
  'h1', 'h2', 'h3', 'th', 'button', '.el-button', 'label',
  '.el-form-item__label', '[placeholder]', '.el-tabs__item', '.el-tag', '.el-alert__title',
].join(',')
const WARN_SELECTOR = 'td'
const COMPARE_SELECTOR = 'h1,h2,h3,button,.el-button,.el-form-item__label' // C5 비교(셸 크롬 포함, .el-pagination 제외 — H-7)
const HANGUL_RE = /[가-힣]/
const DIALOG_SAMPLES = [
  { route: '/assets/server/hosts', text: ['삭제', 'Delete'] },
  { route: '/assets/databases', text: ['삭제', 'Delete'] },
  { route: '/notify/templates', text: ['삭제', 'Delete'] },
]

// ---------------------------------------------------------------------------
// playwright 해석 (H-5 — createRequire 고정, 위치 독립)
// ---------------------------------------------------------------------------

let chromium
{
  // H-5: createRequire 기준점을 web/package.json 으로 고정해 web/node_modules/playwright 를
  // 향하게 한다(위치 독립). 후보 2개 — 본 파일이 web/scripts/ 또는 루트 scripts/ 어디에 있든 동작.
  const errors = []
  for (const candidate of ['../package.json', '../web/package.json']) {
    const pkgUrl = new URL(candidate, import.meta.url)
    if (!existsSync(fileURLToPath(pkgUrl))) continue
    try {
      ;({ chromium } = createRequire(pkgUrl)('playwright'))
      break
    } catch (error) {
      errors.push(`${candidate}: ${error.message.split('\n')[0]}`)
    }
  }
  if (!chromium) {
    console.error('[환경 미비] web/node_modules 에 playwright 가 없다.')
    if (errors.length) console.error(`  ${errors.join(' / ')}`)
    console.error('준비 절차: cd web && npm install --no-save playwright && npx playwright install chromium')
    console.error('(package.json 오염 없음 — --no-save 설치. npm install 시 소멸할 수 있으므로 재실행 필요)')
    process.exit(2)
  }
}

let browser
try {
  browser = await chromium.launch()
} catch (error) {
  console.error(`[환경 미비] chromium 실행 실패: ${error.message}`)
  console.error('준비 절차: cd web && npx playwright install chromium')
  process.exit(2)
}

// ---------------------------------------------------------------------------
// 실행
// ---------------------------------------------------------------------------

const log = []
const report = (line) => { console.log(line); log.push(line) }

const context = await browser.newContext({ viewport: { width: 1600, height: 900 } })
await context.addInitScript(([initLocale, initToken]) => {
  window.localStorage.setItem('ops-admin-display-locale', initLocale)
  if (initToken) {
    window.localStorage.setItem('ops-admin-token', initToken)
    window.localStorage.setItem('ops-admin-token-expires-at', String(Date.now() + 86400_000))
  }
}, [locale, token])

const page = await context.newPage()
mkdirSync(outDir, { recursive: true })

const slug = (route) => route.replace(/^\//, '').replaceAll('/', '-') || 'root'
const settle = (ms = 1500) => page.waitForTimeout(ms)

let serverUp = true
try {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded', timeout: 15_000 })
} catch {
  serverUp = false
}

if (!serverUp) {
  report(`[중단] dev server 접속 불가: ${baseUrl}`)
  report('축소 없음 — 스모크 미실행. 준비: cd web && npm run dev (필요 시 backend 기동)')
  await browser.close()
  process.exit(3)
}

// 실제 로그인(선택)
let authNote = token ? 'token 주입' : '미인증(축소) — 인증 라우트는 /login 으로 리다이렉트될 수 있음'
if (user && pass) {
  try {
    await page.fill('input[placeholder]:not([type=password])', user)
    await page.fill('input[type=password]', pass)
    await page.click('.submit-btn')
    await page.waitForLoadState('networkidle').catch(() => {})
    await settle(1000)
    authNote = page.url().includes('/login') ? '로그인 실패 — 미인증 축소' : '로그인 폼 성공'
  } catch (error) {
    authNote = `로그인 시도 실패(${error.message.split('\n')[0]}) — 미인증 축소`
  }
}
report(`== smoke-i18n locale=${locale} base=${baseUrl} ==`)
report(`인증: ${authNote}`)

const results = []
for (const route of routes) {
  const entry = { route, redirected: false, fail: [], warn: [], samples: [] }
  try {
    await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded', timeout: 15_000 })
    await settle()
  } catch (error) {
    entry.fail.push(`이동 실패: ${error.message.split('\n')[0]}`)
    results.push(entry)
    continue
  }
  const finalPath = new URL(page.url()).pathname
  if (finalPath !== route && finalPath !== `${route}/`) {
    entry.redirected = true
    entry.samples.push(`→ ${finalPath} 리다이렉트`)
  }
  const collect = async (selector) => page.evaluate((sel) => {
    const seen = new Set()
    const texts = []
    for (const el of document.querySelectorAll(sel)) {
      const text = (el.getAttribute('placeholder') ?? el.textContent ?? '').trim()
      if (text && !seen.has(text)) { seen.add(text); texts.push(text) }
    }
    return texts
  }, selector)

  const failTexts = await collect(FAIL_SELECTOR)
  const warnTexts = await collect(WARN_SELECTOR)
  entry.fail = failTexts.filter((t) => HANGUL_RE.test(t))
  entry.warn = warnTexts.filter((t) => HANGUL_RE.test(t))
  entry.compareTexts = await collect(COMPARE_SELECTOR)
  await page.screenshot({ path: join(outDir, `${locale}-${slug(route)}.png`), fullPage: false })
  results.push(entry)
}

// C6 — ElementPlus pagination locale (S1 이후 활성. baseline 부재 시 실패는 WARN 강등)
const c6 = { done: false, ok: null, detail: 'pagination 미발견(미인증/빈 데이터 — 축소)' }
const firstFull = results.find((r) => !r.redirected)
if (firstFull) {
  try {
    await page.goto(`${baseUrl}/dashboard`, { waitUntil: 'domcontentloaded', timeout: 15_000 })
    await settle(2500)
    const paginationText = await page.evaluate(() => document.querySelector('.el-pagination')?.textContent?.trim() || '')
    if (paginationText) {
      c6.done = true
      c6.ok = locale === 'ko-KR' ? paginationText.includes('총') : paginationText.includes('Total')
      c6.detail = paginationText.slice(0, 60)
    }
  } catch { /* 유지: 미발견 축소 */ }
}

// C5 — KO baseline 비교 (baseline 은 S1 페이즈가 1회 캡처)
const baselinePath = join(ROOT, 'en-extract/ko-baseline.json')
let c5 = { status: 'skip', detail: 'baseline 부재 — S1 페이즈에서 1회 캡처 예정(G1은 캡처하지 않음)' }
if (locale === 'ko-KR' && captureBaseline) {
  // S1 전용: baseline 1회 캡처(--capture-baseline). G1/S1 외 사용 금지(bundle §4.2 — 캡처는 1회·단일 주체).
  const payload = {
    description: 'KO 무회귀 기준선 — S1 완료 시점 1회 캡처(bundle §4.2 H-7). .el-pagination 제외',
    capturedAt: new Date().toISOString(),
    baseUrl,
    selector: COMPARE_SELECTOR,
    routes: Object.fromEntries(results.map((r) => [r.route, r.compareTexts || []])),
  }
  writeFileSync(baselinePath, JSON.stringify(payload, null, 2) + '\n')
  c5 = { status: 'captured', detail: `baseline 기록: ${baselinePath}` }
} else if (locale === 'ko-KR' && existsSync(baselinePath)) {
  const baseline = JSON.parse(await import('node:fs').then((m) => m.readFileSync(baselinePath, 'utf8')))
  const base = baseline.routes || {}
  const diffs = []
  for (const entry of results) {
    const expected = base[entry.route]
    if (!expected) { diffs.push(`${entry.route}: baseline 항목 없음`); continue }
    if (JSON.stringify(expected) !== JSON.stringify(entry.compareTexts || [])) {
      diffs.push(`${entry.route}: 텍스트 불일치 (baseline ${expected.length}건 vs 현재 ${(entry.compareTexts || []).length}건)`)
    }
  }
  c5 = diffs.length
    ? { status: 'fail', detail: `${diffs.length} 라우트 불일치`, diffs: diffs.slice(0, 10) }
    : { status: 'pass', detail: `${results.length} 라우트 100% 일치` }
}

// --dialogs — ElMessageBox 샘플링 (C4 사해 셀렉터 대체)
const dialogResults = []
if (dialogs) {
  for (const sample of DIALOG_SAMPLES) {
    const record = { route: sample.route, opened: false, message: null }
    try {
      await page.goto(`${baseUrl}${sample.route}`, { waitUntil: 'domcontentloaded', timeout: 15_000 })
      await settle()
      const clicked = await page.evaluate((needles) => {
        const buttons = [...document.querySelectorAll('button, .el-button')]
        const target = buttons.find((b) => needles.some((n) => b.textContent.includes(n)))
        if (target) { target.click(); return true }
        return false
      }, sample.text)
      if (clicked) {
        await page.waitForSelector('.el-message-box', { timeout: 3000 }).catch(() => {})
        const message = await page.evaluate(() => document.querySelector('.el-message-box__message')?.textContent?.trim() || null)
        record.opened = Boolean(message)
        record.message = message
        await page.evaluate(() => document.querySelector('.el-message-box__btns .el-button')?.click()).catch(() => {})
      }
    } catch (error) {
      record.message = `샘플링 실패: ${error.message.split('\n')[0]}`
    }
    dialogResults.push(record)
  }
}

// ---------------------------------------------------------------------------
// 판정
// ---------------------------------------------------------------------------

let koreanTotal = 0
for (const entry of results) {
  const tag = entry.redirected ? 'REDIRECT' : 'OK'
  koreanTotal += entry.fail.length
  report(`  [${tag}] ${entry.route} — FAIL셀렉터 한글 ${entry.fail.length}건 / td WARN ${entry.warn.length}건${entry.fail.length ? ` 예: ${entry.fail.slice(0, 5).join(' | ')}` : ''}${entry.warn.length ? ` td예: ${entry.warn.slice(0, 3).join(' | ')}` : ''}`)
}
report(`C4 대상 셀렉터 한글 총계: ${koreanTotal}건`)
report(`C6 ElementPlus pagination: ${c6.done ? (c6.ok ? `통과(${locale}: ${c6.detail})` : `실패(${locale}: ${c6.detail})`) : `스킵 — ${c6.detail}`}`)
if (dialogResults.length) {
  for (const d of dialogResults) report(`dialogs 샘플 ${d.route}: ${d.opened ? `열림 — "${(d.message || '').slice(0, 80)}"` : `미확인(${d.message || '버튼 부재/미인증 — 축소'})`}`)
}
if (locale === 'ko-KR') report(`C5 KO baseline: ${c5.status.toUpperCase()} — ${c5.detail}${c5.diffs ? ` 예: ${c5.diffs[0]}` : ''}`)

// baseline 부재 시점의 C6 실패는 S1 전 위양성이므로 WARN(통과 처리, 사유 명시) — 은폐 아님
let exitCode = 0
if (locale === 'en-US' && koreanTotal > 0) exitCode = 1
if (locale === 'ko-KR' && c5.status === 'fail') exitCode = 1
if (c6.done && c6.ok === false && !(locale === 'ko-KR' && c5.status === 'skip')) exitCode = exitCode || 1
if (c6.done && c6.ok === false && locale === 'ko-KR' && c5.status === 'skip') {
  report('[WARN] C6 KO 실패 — ElementPlus locale 미설정(S1 판단 B 대상). S1 전 위양성으로 WARN 처리.')
}

const summary = { locale, baseUrl, auth: authNote, routes: results.length, koreanTotal, c5, c6, exitCode }
writeFileSync(join(outDir, `smoke-summary-${locale}.json`), JSON.stringify({ ...summary, results, dialogResults }, null, 2) + '\n')
report(`RESULT: ${exitCode === 0 ? 'PASS' : 'FAIL'} (exit ${exitCode}) — 스크린샷·요약: ${outDir}`)
writeFileSync(join(outDir, `smoke-report-${locale}.log`), log.join('\n') + '\n')

await browser.close()
process.exit(exitCode)
