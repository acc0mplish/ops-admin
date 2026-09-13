import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { queryMonitorDashboardPanel } from '../api/monitor'
import { mt } from '../utils/monitor-i18n'

// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 1/2.
// 패널 쿼리 엔진과 렌더 어휘를 뷰에서 이동했다(원본 좌표는 각 함수 앞 주석).
// 상태 소유는 뷰에 남고 반응형 참조는 인자로 주입받는다(§5 #11 행동 보존+재배선).
export function useMonitorDashboardPanels({ panels, activePanels, selectedDatasourceId, timeRangeSeconds, lastRefreshAt, isK8sDashboard }) {
  const panelResults = reactive({})
  const panelPending = reactive({})
  let panelRefreshVersion = 0
  const panelResultCache = new Map()
  const PANEL_QUERY_CONCURRENCY = 4
  const PANEL_CACHE_TTL = 15 * 1000

  // 원본 MonitorDashboard.vue:198-202
  function metricText(metric) {
    return Object.entries(metric || {})
      .map(([key, value]) => `${key}="${value}"`)
      .join(', ')
  }

  // 원본 MonitorDashboard.vue:204-207
  function metricName(metric) {
    if (!metric) return 'metric'
    return metric.instance || metric.pod || metric.namespace || metric.job || metric.__name__ || 'metric'
  }

  // 원본 MonitorDashboard.vue:209-213
  function numberValue(row) {
  	const raw = row?.value?.[1] ?? row?.values?.[row.values.length - 1]?.[1]
  	const value = Number(raw)
    return Number.isFinite(value) ? value : 0
  }

  // 원본 MonitorDashboard.vue:215-220 — §3-0 주석 포함 원문 이동
  // §3-0: unit 토큰('일'/'개'/'대'/'회')은 formatByUnit의 비교 피연산자이므로 값 자체는
  // 원문 byte 불변. 화면 표시 접미만 이 경유 번역(KO locale은 원문 byte 그대로 반환).
  function unitText(unit) {
    const key = ({ '일': 'unitDay', '개': 'unitCount', '대': 'unitHost', '회': 'unitTimes' })[unit]
    return key ? mt(key) : unit
  }

  // 원본 MonitorDashboard.vue:222-239
  function formatByUnit(value, unit = '') {
    if (!Number.isFinite(value)) return '-'
    if (unit === 'B/s') {
      const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s']
      let current = Math.abs(value)
      let index = 0
      while (current >= 1024 && index < units.length - 1) {
        current /= 1024
        index += 1
      }
      const signed = value < 0 ? -current : current
      return `${signed.toFixed(current >= 100 ? 0 : current >= 10 ? 1 : 2)} ${units[index]}`
    }
    if (unit === '일') return `${value.toFixed(value >= 10 ? 0 : 1)}${unitText(unit)}`
    if (unit === '%') return `${value.toFixed(value >= 10 ? 1 : 2).replace(/\.?0+$/, '')}%`
    if (unit && unit !== '개' && unit !== '대') return `${value.toFixed(value >= 100 ? 0 : 2).replace(/\.?0+$/, '')}${unitText(unit)}`
    return value.toLocaleString(undefined, { maximumFractionDigits: value >= 100 ? 0 : 2 })
  }

  // 원본 MonitorDashboard.vue:241-243
  function panelRows(panel) {
    return panelResults[panel.id]?.result || []
  }

  // 원본 MonitorDashboard.vue:245-251
  function panelValue(panel) {
  	const value = numberValue(panelRows(panel)[0])
    if (!Number.isFinite(value)) return '-'
    const formatted = formatByUnit(value, panel.unit)
    const displayUnit = unitText(panel.unit)
    return panel.unit && displayUnit && formatted.endsWith(displayUnit) ? formatted.slice(0, -displayUnit.length).trim() : formatted
  }

  // 원본 MonitorDashboard.vue:253-257
  function panelTrend(panel) {
  	const firstSeries = panelRows(panel)[0]
  	if (firstSeries?.values?.length) return firstSeries.values.map((item) => Number(item?.[1])).filter(Number.isFinite)
  	return panelRows(panel).slice(0, 24).map((item) => numberValue(item))
  }

  // 원본 MonitorDashboard.vue:259-265
  function sparklinePoints(panel) {
    const values = panelTrend(panel)
    if (!values.length) return ''
    const max = Math.max(...values)
    const min = Math.min(...values)
    return trendPoints(values, min, max)
  }

  // 원본 MonitorDashboard.vue:267-274
  function trendPoints(values, min, max) {
    const range = max - min || 1
    return values.map((value, index) => {
      const x = values.length === 1 ? 100 : (index / (values.length - 1)) * 100
      const y = 48 - ((value - min) / range) * 40
      return `${x},${y}`
    }).join(' ')
  }

  // 원본 MonitorDashboard.vue:276
  const lineColors = ['#3b82f6', '#14b8a6', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4', '#84cc16', '#ec4899']

  // 원본 MonitorDashboard.vue:278-292
  function panelLineSeries(panel) {
    const series = panelRows(panel)
      .map((row) => ({ name: metricName(row.metric), values: (row.values || []).map((item) => Number(item?.[1])).filter(Number.isFinite) }))
      .filter((item) => item.values.length)
      .slice(0, 8)
    const allValues = series.flatMap((item) => item.values)
    if (!allValues.length) return []
    const min = Math.min(...allValues)
    const max = Math.max(...allValues)
    return series.map((item, index) => ({
      ...item,
      color: lineColors[index % lineColors.length],
      points: trendPoints(item.values, min, max)
    }))
  }

  // 원본 MonitorDashboard.vue:294-297
  function sparklineAreaPoints(panel) {
    const points = panelLineSeries(panel)[0]?.points || sparklinePoints(panel)
    return points ? `0,52 ${points} 100,52` : ''
  }

  // 원본 MonitorDashboard.vue:299-310
  function panelStats(panel) {
    const values = panelTrend(panel)
    if (!values.length) return { min: '-', max: '-', avg: '-' }
    const min = Math.min(...values)
    const max = Math.max(...values)
    const avg = values.reduce((sum, value) => sum + value, 0) / values.length
    return {
      min: formatByUnit(min, panel.unit),
      max: formatByUnit(max, panel.unit),
      avg: formatByUnit(avg, panel.unit)
    }
  }

  // 원본 MonitorDashboard.vue:312-314
  function panelChartLabel(chartType) {
    return ({ stat: 'Metric', gauge: 'Gauge', bar: 'Ranking', line: 'Trend', table: mt('detail') })[chartType] || chartType
  }

  // 원본 MonitorDashboard.vue:316-323
  function barRows(panel) {
    const rows = panelRows(panel)
      .map((row) => ({ name: metricName(row.metric), value: numberValue(row), displayValue: formatByUnit(numberValue(row), panel.unit) }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 8)
    const max = Math.max(...rows.map((item) => item.value), 1)
    return rows.map((item) => ({ ...item, percent: Math.max(4, (item.value / max) * 100) }))
  }

  // 원본 MonitorDashboard.vue:325-329
  function gaugePercent(panel) {
  	const value = numberValue(panelRows(panel)[0])
    if (!Number.isFinite(value)) return 0
    return Math.max(0, Math.min(100, value))
  }

  // 원본 MonitorDashboard.vue:331-337
  function panelSpan(panel) {
    const span = Number(panel.span || 12)
    if (isK8sDashboard.value) {
      return Math.max(1, Math.min(6, Math.round(span / 4)))
    }
    return Math.max(1, Math.min(4, Math.round(span / 6)))
  }

  // 원본 MonitorDashboard.vue:339-344
  function panelState(panel) {
    if (panel.status !== 1) return mt('stateDisabled')
    if (panelResults[panel.id]?.error) return mt('stateQueryFailed')
    if (!panelRows(panel).length) return mt('stateNoData')
    return mt('stateLive')
  }

  // 원본 MonitorDashboard.vue:346-351
  function panelStateKey(panel) {
    if (panel.status !== 1) return 'disabled'
    if (panelResults[panel.id]?.error) return 'danger'
    if (!panelRows(panel).length) return 'warning'
    return 'healthy'
  }

  // 원본 MonitorDashboard.vue:353-358
  function panelStateType(panel) {
    if (panel.status !== 1) return 'info'
    if (panelResults[panel.id]?.error) return 'danger'
    if (!panelRows(panel).length) return 'warning'
    return 'success'
  }

  // 원본 MonitorDashboard.vue:360-362
  function panelResultCount(panel) {
    return panelRows(panel).length
  }

  // 원본 MonitorDashboard.vue:364-368
  function panelDisplayValue(panel) {
  	const value = numberValue(panelRows(panel)[0])
    if (!Number.isFinite(value)) return '-'
    return formatByUnit(value, panel.unit)
  }

  // 원본 MonitorDashboard.vue:370-377
  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;')
  }

  // 원본 MonitorDashboard.vue:628-638
  function panelQueryPayload(id) {
  	const endAt = Math.floor(Date.now() / 1000)
  	const startAt = endAt - timeRangeSeconds.value
  	return {
  		id,
  		datasourceId: selectedDatasourceId.value,
  		startAt,
  		endAt,
  		stepSeconds: Math.max(15, Math.ceil(timeRangeSeconds.value / 120))
  	}
  }

  // 원본 MonitorDashboard.vue:640-642
  function panelCacheKey(panel) {
    return `${selectedDatasourceId.value}:${timeRangeSeconds.value}:${panel.id}`
  }

  // 원본 MonitorDashboard.vue:602-609
  async function refreshPanel(row) {
    if (!selectedDatasourceId.value) {
      ElMessage.warning(mt('configureDatasourceFirst'))
      return
    }
    await loadPanel(row, { force: true })
    lastRefreshAt.value = new Date()
  }

  // 원본 MonitorDashboard.vue:611-617
  async function refreshProblemPanels() {
    const items = activePanels.value.filter((panel) => ['danger', 'warning'].includes(panelStateKey(panel)))
    if (!items.length) return ElMessage.success(mt('noProblemPanels'))
    const version = ++panelRefreshVersion
    await runPanelQueue(items, version, true)
    if (version === panelRefreshVersion) lastRefreshAt.value = new Date()
  }

  // 원본 MonitorDashboard.vue:644-665
  async function loadPanel(panel, { force = true, version = panelRefreshVersion } = {}) {
    const key = panelCacheKey(panel)
    const cached = panelResultCache.get(key)
    if (!force && cached && cached.expiresAt > Date.now()) {
      panelResults[panel.id] = cached.data
      return
    }

    panelPending[panel.id] = true
    try {
      const data = await queryMonitorDashboardPanel(panelQueryPayload(panel.id))
      if (version !== panelRefreshVersion) return
      panelResults[panel.id] = data
      panelResultCache.set(key, { data, expiresAt: Date.now() + PANEL_CACHE_TTL })
    } catch (error) {
      if (version === panelRefreshVersion) {
        panelResults[panel.id] = { error: error.message || 'query failed' }
      }
    } finally {
      if (version === panelRefreshVersion) delete panelPending[panel.id]
    }
  }

  // 원본 MonitorDashboard.vue:667-677
  async function runPanelQueue(items, version, force) {
    let cursor = 0
    const worker = async () => {
      while (cursor < items.length && version === panelRefreshVersion) {
        const panel = items[cursor]
        cursor += 1
        await loadPanel(panel, { force, version })
      }
    }
    await Promise.all(Array.from({ length: Math.min(PANEL_QUERY_CONCURRENCY, items.length) }, worker))
  }

  // 원본 MonitorDashboard.vue:679-702
  async function refreshAllPanels({ progressive = false, force = true } = {}) {
    if (!selectedDatasourceId.value) {
      const message = mt('configureDatasourceFirst')
      for (const panel of panels.value.filter((item) => item.status === 1)) panelResults[panel.id] = { error: message }
      return
    }
    const enabledPanels = panels.value.filter((item) => item.status === 1)
    const version = ++panelRefreshVersion
    const firstScreenPanels = enabledPanels.slice(0, 6)
    const remainingPanels = enabledPanels.slice(6)
    const loadRemaining = () => runPanelQueue(remainingPanels, version, force)

    if (progressive) {
      await runPanelQueue(firstScreenPanels, version, force)
      if (version === panelRefreshVersion && remainingPanels.length) {
        window.setTimeout(() => { void loadRemaining() }, 0)
      }
      return
    }

    await runPanelQueue(firstScreenPanels, version, force)
    if (version === panelRefreshVersion) await loadRemaining()
    if (version === panelRefreshVersion) lastRefreshAt.value = new Date()
  }

  return {
    panelResults,
    panelPending,
    metricText,
    metricName,
    numberValue,
    unitText,
    formatByUnit,
    panelRows,
    panelValue,
    panelTrend,
    sparklinePoints,
    panelLineSeries,
    sparklineAreaPoints,
    panelStats,
    panelChartLabel,
    barRows,
    gaugePercent,
    panelSpan,
    panelState,
    panelStateKey,
    panelStateType,
    panelResultCount,
    panelDisplayValue,
    escapeHtml,
    refreshPanel,
    refreshProblemPanels,
    refreshAllPanels
  }
}
