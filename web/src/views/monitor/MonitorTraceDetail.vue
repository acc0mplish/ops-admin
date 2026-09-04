<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { queryMonitorDatasourceOptions, queryMonitorTraceDetail } from '../../api/monitor'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const trace = ref(null)
const datasource = ref(null)
const selectedSpanId = ref('')

function numeric(value) { return Number(value || 0) }
function formatDuration(value) {
  const microseconds = numeric(value)
  if (microseconds >= 1000000) return `${(microseconds / 1000000).toFixed(2)} s`
  if (microseconds >= 1000) return `${(microseconds / 1000).toFixed(2)} ms`
  return `${microseconds} μs`
}
function formatTime(value) {
  const date = new Date(numeric(value) / 1000)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}
function isError(span) {
  return (span.tags || []).some((tag) => tag.key === 'error' && String(tag.value) !== 'false') || (span.tags || []).some((tag) => tag.key === 'http.status_code' && numeric(tag.value) >= 400)
}

const traceStart = computed(() => Math.min(...(trace.value?.spans || []).map((span) => numeric(span.startTime))))
const traceEnd = computed(() => Math.max(...(trace.value?.spans || []).map((span) => numeric(span.startTime) + numeric(span.duration))))
const traceDuration = computed(() => Math.max(1, traceEnd.value - traceStart.value))
const services = computed(() => [...new Set(Object.values(trace.value?.processes || {}).map((process) => process.serviceName).filter(Boolean))])
const errorCount = computed(() => (trace.value?.spans || []).filter(isError).length)
const spanRows = computed(() => {
  const spans = trace.value?.spans || []
  const byID = new Map(spans.map((span) => [span.spanID, span]))
  const depthCache = new Map()
  const getParent = (span) => (span.references || []).find((reference) => reference.refType === 'CHILD_OF')?.spanID
  const getDepth = (span, seen = new Set()) => {
    if (depthCache.has(span.spanID)) return depthCache.get(span.spanID)
    const parentID = getParent(span)
    if (!parentID || seen.has(parentID) || !byID.has(parentID)) return 0
    const result = Math.min(8, getDepth(byID.get(parentID), new Set([...seen, span.spanID])) + 1)
    depthCache.set(span.spanID, result)
    return result
  }
  return spans.map((span) => {
    const start = numeric(span.startTime)
    const duration = numeric(span.duration)
    return {
      ...span,
      service: trace.value?.processes?.[span.processID]?.serviceName || '-',
      process: trace.value?.processes?.[span.processID] || {},
      depth: getDepth(span),
      offset: Math.max(0, (start - traceStart.value) / traceDuration.value * 100),
      width: Math.max(1.2, duration / traceDuration.value * 100),
      error: isError(span)
    }
  }).sort((left, right) => numeric(left.startTime) - numeric(right.startTime))
})
const selectedSpan = computed(() => spanRows.value.find((span) => span.spanID === selectedSpanId.value) || spanRows.value[0])
const ticks = computed(() => [0, 25, 50, 75, 100].map((position) => ({ position, value: formatDuration(traceDuration.value * position / 100) })))

async function load() {
  const datasourceID = Number(route.query.datasourceId)
  if (!datasourceID || !route.params.traceId) {
    ElMessage.warning('Jaeger Datasource 또는 Trace ID가 없습니다')
    router.replace('/monitor/traces')
    return
  }
  loading.value = true
  try {
    const options = await queryMonitorDatasourceOptions()
    datasource.value = (options || []).find((item) => item.id === datasourceID)
    trace.value = await queryMonitorTraceDetail({ datasourceId: datasourceID, traceId: route.params.traceId })
    selectedSpanId.value = trace.value?.spans?.[0]?.spanID || ''
  } finally { loading.value = false }
}
async function copyTraceID() {
  try { await navigator.clipboard.writeText(trace.value?.traceID || route.params.traceId); ElMessage.success('Trace ID를 복사했습니다.') } catch { ElMessage.info('Trace ID를 직접 복사하십시오.') }
}
function back() { router.back() }

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="trace-detail-page">
    <div class="detail-topbar">
      <el-button text class="back-button" @click="back">‹ Trace로 돌아가기</el-button>
      <div class="title-wrap"><h2>{{ selectedSpan?.operationName || 'Trace 상세' }}</h2><span class="source-name">{{ datasource?.name || 'Jaeger' }}</span></div>
      <el-button plain @click="copyTraceID">Trace ID 복사</el-button>
    </div>

    <template v-if="trace">
      <div class="trace-ident"><span>Trace ID</span><code>{{ trace.traceID }}</code></div>
      <section class="summary-grid">
        <div class="summary-card start-card"><i class="summary-icon">◷</i><span>시작 시각</span><strong>{{ formatTime(traceStart) }}</strong></div>
        <div class="summary-card duration-card"><i class="summary-icon">⌁</i><span>총 소요 시간</span><strong>{{ formatDuration(traceDuration) }}</strong><small>End-to-End 호출 시간</small></div>
        <div class="summary-card service-card"><i class="summary-icon">⌘</i><span>Service</span><strong>{{ services.length }}</strong><small>{{ services.join(' · ') }}</small></div>
        <div class="summary-card span-card"><i class="summary-icon">◎</i><span>Span</span><strong>{{ spanRows.length }}</strong><small>최대 깊이 {{ Math.max(0, ...spanRows.map((item) => item.depth)) + 1 }}</small></div>
        <div class="summary-card" :class="{ danger: errorCount }"><i class="summary-icon">!</i><span>오류</span><strong>{{ errorCount }}</strong><small>{{ errorCount ? '빨간색 표시 Span을 확인하십시오.' : '오류 표시 없음' }}</small></div>
      </section>

      <section class="waterfall-card">
        <div class="section-head"><div><h3>호출 Timeline</h3><p>Span을 클릭하면 Label, Process와 Log 상세를 확인할 수 있습니다.</p></div><div class="legend"><i class="normal"></i>정상 <i class="error"></i>오류</div></div>
        <div class="timeline-scroll">
          <div class="timeline-grid">
            <div class="tree-head">Service / Operation</div>
            <div class="ruler"><span v-for="tick in ticks" :key="tick.position" :style="{ left: `${tick.position}%` }">{{ tick.value }}</span></div>
            <template v-for="span in spanRows" :key="span.spanID">
              <button class="span-name" :class="{ selected: selectedSpan?.spanID === span.spanID, error: span.error }" :style="{ '--depth': span.depth }" @click="selectedSpanId = span.spanID">
                <b>{{ span.service }}</b><span>{{ span.operationName }}</span><em>{{ span.error ? '오류' : '성공' }}</em>
              </button>
              <button class="span-track" :class="{ selected: selectedSpan?.spanID === span.spanID }" @click="selectedSpanId = span.spanID">
                <i v-for="tick in ticks.slice(1, -1)" :key="tick.position" class="guide" :style="{ left: `${tick.position}%` }"></i>
                <span class="span-bar" :class="{ error: span.error }" :style="{ left: `${span.offset}%`, width: `${Math.min(span.width, 100 - span.offset)}%` }"><em>{{ formatDuration(span.duration) }}</em></span>
              </button>
            </template>
          </div>
        </div>
      </section>

      <section v-if="selectedSpan" class="inspector-card">
        <div class="inspector-title"><div><span class="eyebrow">선택한 Span</span><h3>{{ selectedSpan.operationName }}</h3><p>{{ selectedSpan.service }} · {{ formatDuration(selectedSpan.duration) }} · {{ formatTime(selectedSpan.startTime) }}</p></div><div class="selection-actions"><span class="span-id">{{ selectedSpan.spanID }}</span><el-tag :type="selectedSpan.error ? 'danger' : 'success'">{{ selectedSpan.error ? '오류' : '성공' }}</el-tag></div></div>
        <div class="info-columns">
          <div><h4>Label</h4><div v-if="selectedSpan.tags?.length" class="key-values"><div v-for="tag in selectedSpan.tags" :key="tag.key"><span>{{ tag.key }}</span><code>{{ tag.value }}</code></div></div><el-empty v-else description="Label이 없습니다" :image-size="44" /></div>
          <div><h4>Process</h4><div class="service-box"><b>{{ selectedSpan.process.serviceName || selectedSpan.service }}</b><div v-for="tag in selectedSpan.process.tags || []" :key="tag.key"><span>{{ tag.key }}</span><code>{{ tag.value }}</code></div></div><h4 class="logs-title">Log Event</h4><div v-if="selectedSpan.logs?.length" class="logs"><div v-for="(log, index) in selectedSpan.logs" :key="index"><time>{{ formatTime(log.timestamp) }}</time><span v-for="field in log.fields || []" :key="field.key">{{ field.key }}={{ field.value }}</span></div></div><el-empty v-else description="Log Event가 없습니다" :image-size="44" /></div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.trace-detail-page { min-height: calc(100vh - 130px); padding: 26px; color: #1d2d4a; background: radial-gradient(circle at 8% 0%, #e9f3ff 0, transparent 25%), #f4f7fb; }.detail-topbar { display: flex; align-items: center; gap: 16px; min-height: 42px; }.back-button { padding-left: 0; color: #4771ad; }.title-wrap { flex: 1; display: flex; align-items: center; gap: 10px; }.title-wrap h2 { margin: 0; font-size: 23px; letter-spacing: -.3px; }.source-name { padding: 4px 10px; color: #3767b7; font-size: 12px; background: #e8f2ff; border: 1px solid #d5e6fc; border-radius: 999px; }.trace-ident { display: flex; gap: 10px; align-items: center; margin: 17px 0; color: #71829b; font-size: 13px; }.trace-ident code { padding: 5px 8px; color: #245dcc; background: rgba(255,255,255,.72); border-radius: 6px; word-break: break-all; }.summary-grid { display: grid; grid-template-columns: 1.35fr repeat(4, 1fr); gap: 14px; }.summary-card, .waterfall-card, .inspector-card { background: rgba(255,255,255,.94); border: 1px solid #e0e9f5; border-radius: 16px; box-shadow: 0 10px 28px rgba(41, 75, 124, .07); }.summary-card { position: relative; display: flex; flex-direction: column; gap: 5px; min-width: 0; min-height: 90px; padding: 17px 18px; overflow: hidden; }.summary-card::after { position: absolute; right: -24px; bottom: -32px; width: 100px; height: 100px; content: ''; background: #ecf5ff; border-radius: 50%; }.summary-card.duration-card::after { background: #e8fbfa; }.summary-card.service-card::after { background: #f1edff; }.summary-card.span-card::after { background: #fff6e7; }.summary-card.danger::after { background: #fff0ef; }.summary-icon { position: absolute; right: 16px; top: 15px; z-index: 1; display: grid; width: 28px; height: 28px; color: #4382d4; font-size: 18px; font-style: normal; font-weight: 700; place-items: center; background: #edf5ff; border-radius: 9px; }.duration-card .summary-icon { color: #15959c; background: #e8fbfa; }.service-card .summary-icon { color: #725ac7; background: #f0ecff; }.span-card .summary-icon { color: #d58716; background: #fff6e5; }.danger .summary-icon { color: #d94e55; background: #fff0ef; }.summary-card > span, .summary-card small { position: relative; z-index: 1; color: #8090a8; font-size: 12px; }.summary-card strong { position: relative; z-index: 1; color: #172b4d; font-size: 20px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }.summary-card.danger strong { color: #dc4c4c; }.summary-card small { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }.waterfall-card { margin-top: 20px; overflow: hidden; }.section-head, .inspector-title { display: flex; align-items: center; justify-content: space-between; padding: 18px 21px; border-bottom: 1px solid #e9eff7; }.section-head h3, .inspector-title h3 { margin: 0; font-size: 16px; }.section-head p, .inspector-title p { margin: 5px 0 0; color: #7b8ca5; font-size: 12px; }.legend { display: flex; gap: 7px; align-items: center; color: #7b8ca5; font-size: 12px; }.legend i { width: 11px; height: 11px; border-radius: 4px; }.legend .normal { background: #28b5bd; }.legend .error { margin-left: 8px; background: #e45e5e; }.timeline-scroll { padding: 0 10px 10px; overflow-x: auto; background: #f9fbfe; }.timeline-grid { display: grid; grid-template-columns: minmax(300px, 32%) minmax(660px, 68%); min-width: 1020px; }.tree-head, .ruler { height: 50px; padding: 16px 18px; box-sizing: border-box; color: #637895; font-size: 13px; font-weight: 600; background: transparent; border-bottom: 1px solid #e7edf6; }.ruler { position: relative; border-left: 1px solid #e7edf6; }.ruler span { position: absolute; top: 15px; font-size: 11px; color: #8293ac; transform: translateX(-50%); }.ruler span:first-child { transform: none; }.ruler span:last-child { transform: translateX(-100%); }.span-name, .span-track { min-height: 57px; margin-top: 8px; border: 1px solid #e8eef6; background: #fff; cursor: pointer; text-align: left; transition: box-shadow .16s ease, transform .16s ease, border-color .16s ease; }.span-name { display: flex; gap: 8px; align-items: center; padding: 8px 11px 8px calc(14px + var(--depth) * 18px); overflow: hidden; border-right: 0; border-radius: 10px 0 0 10px; }.span-name::before { width: 4px; height: 29px; content: ''; background: #31b7be; border-radius: 3px; flex: none; }.span-name b { max-width: 112px; padding: 3px 6px; color: #286f76; font-size: 11px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; background: #e9f9f9; border-radius: 5px; }.span-name span { flex: 1; color: #516680; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }.span-name em { padding: 3px 6px; color: #20976b; font-size: 10px; font-style: normal; background: #e9faf1; border-radius: 999px; }.span-name.selected, .span-track.selected { z-index: 1; background: #f2f8ff; border-color: #94c5ff; box-shadow: 0 5px 14px rgba(61, 137, 223, .12); }.span-track { position: relative; border-left: 1px dashed #dbe5f1; border-radius: 0 10px 10px 0; }.span-name:hover, .span-track:hover { border-color: #a9caef; }.guide { position: absolute; top: 0; bottom: 0; width: 1px; background: #e9eef5; }.span-bar { position: absolute; top: 21px; height: 15px; min-width: 7px; border-radius: 5px; background: linear-gradient(90deg, #25b8c0, #55d1d2); box-shadow: inset 0 -3px rgba(11, 93, 101, .22), 0 2px 5px rgba(32, 165, 172, .24); }.span-bar.error { background: linear-gradient(90deg, #e95c61, #f38476); box-shadow: inset 0 -3px rgba(131, 27, 27, .2), 0 2px 5px rgba(221, 70, 70, .22); }.span-bar em { position: absolute; top: -16px; right: 0; color: #71849d; font-size: 10px; font-style: normal; white-space: nowrap; opacity: 0; }.span-track:hover .span-bar em, .span-track.selected .span-bar em { opacity: 1; }.inspector-card { margin-top: 20px; }.eyebrow { color: #4080dc; font-size: 12px; }.selection-actions { display: flex; gap: 9px; align-items: center; }.span-id { max-width: 210px; padding: 5px 8px; overflow: hidden; color: #71839d; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; background: #f5f8fc; border-radius: 6px; }.info-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 22px; padding: 20px 21px 25px; }.info-columns > div { min-width: 0; padding: 15px; background: #fafcff; border: 1px solid #eaf0f7; border-radius: 12px; }.info-columns h4 { margin: 0 0 11px; color: #526983; font-size: 13px; }.key-values, .service-box { overflow: hidden; border: 1px solid #e7edf6; border-radius: 8px; background: #fff; }.key-values > div, .service-box > div { display: grid; grid-template-columns: minmax(120px, 40%) 1fr; gap: 10px; padding: 8px 10px; font-size: 12px; border-bottom: 1px solid #eef2f7; }.key-values > div:last-child, .service-box > div:last-child { border-bottom: 0; }.key-values span, .service-box span { color: #71839d; overflow: hidden; text-overflow: ellipsis; }.key-values code, .service-box code { color: #334b6a; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.service-box b { display: block; padding: 10px; color: #26757b; font-size: 13px; background: #ecfbfb; }.logs-title { margin-top: 18px !important; }.logs { display: flex; flex-direction: column; gap: 6px; }.logs > div { display: flex; flex-wrap: wrap; gap: 8px; padding: 8px 10px; color: #516680; font-size: 12px; background: #fff; border: 1px solid #edf1f6; border-radius: 6px; }.logs time { color: #8797ad; }
.span-name.error::before { background: #e45e5e; }
.span-name.error em { color: #d84a51; background: #fff0ef; }
@media (max-width: 900px) { .summary-grid { grid-template-columns: repeat(2, 1fr); }.info-columns { grid-template-columns: 1fr; }.detail-topbar { flex-wrap: wrap; }.span-id { display: none; } }
</style>
