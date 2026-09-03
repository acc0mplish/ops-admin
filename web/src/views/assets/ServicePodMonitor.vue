<script setup>
import { computed, ref, watch } from 'vue'
import { queryAssetServiceWorkloadMetrics } from '../../api/asset'
import { queryK8sWorkloadMetrics } from '../../api/k8s'
import { at } from '../../utils/asset-i18n'
import { currentLocale } from '../../utils/i18n-runtime'

const props = defineProps({ serviceId: Number, clusterId: Number, namespace: String, workloadType: String, workloadName: String, compact: Boolean })
const range = ref('1h')
const loading = ref(false)
const status = ref('idle')
const cpuSeries = ref([])
const memorySeries = ref([])
const wssSeries = ref([])
const networkInSeries = ref([])
const networkOutSeries = ref([])
const cpuHover = ref({ visible: false })
const memoryHover = ref({ visible: false })
const wssHover = ref({ visible: false })
const networkHover = ref({ visible: false })
const colors = ['#3b82f6', '#22b68a', '#f59e0b', '#8b5cf6', '#ef5b63', '#06a6c8', '#75a743', '#d66bb5']

const cpuChart = computed(() => chartModel(cpuSeries.value))
const memoryChart = computed(() => chartModel(memorySeries.value))
const wssChart = computed(() => chartModel(wssSeries.value))
const networkSeries = computed(() => [...networkInSeries.value, ...networkOutSeries.value].map((item) => ({ ...item, name: `${item.direction === 'out' ? at('outbound') : at('inbound')}: ${item.name}` })))
const networkChart = computed(() => chartModel(networkSeries.value))
const metricCards = computed(() => (props.compact ? [
  { key: 'cpu', title: at('cpuCoreUsage'), unit: at('cpuUnit'), chart: cpuChart.value, hover: cpuHover.value },
  { key: 'memory', title: at('memoryUsage'), unit: at('workingSet'), chart: memoryChart.value, hover: memoryHover.value }
] : [
  { key: 'cpu', title: at('cpuCoreUsage'), unit: at('cpuUnit'), chart: cpuChart.value, hover: cpuHover.value },
  { key: 'memory', title: at('memoryUsage'), unit: at('memoryRss'), chart: memoryChart.value, hover: memoryHover.value },
  { key: 'wss', title: at('memoryUsageRate'), unit: at('workingSetLimit'), chart: wssChart.value, hover: wssHover.value },
  { key: 'network', title: at('networkBandwidth'), unit: at('inboundOutbound'), chart: networkChart.value, hover: networkHover.value }
]))
const hasData = computed(() => cpuSeries.value.length || memorySeries.value.length || wssSeries.value.length || networkInSeries.value.length || networkOutSeries.value.length)

function normalizeSeries(series) {
  return (series || []).map((item) => ({ name: item.name || at('unknownPod'), latest: Number(item.latest), points: (item.points || []).map((point) => [Number(point.timestamp) / 1000, Number(point.value)]).filter(([time, value]) => Number.isFinite(time) && Number.isFinite(value)) })).filter((item) => item.points.length)
}
function chartModel(series) {
  const points = series.flatMap((item) => item.points)
  if (!points.length) return { series: [], min: 0, max: 1, start: 0, end: 1 }
  const values = points.map((item) => item[1]); const times = points.map((item) => item[0])
  const min = Math.min(...values); const rawMax = Math.max(...values); const max = rawMax === min ? min + Math.max(1, Math.abs(min) * 0.08) : rawMax
  const start = Math.min(...times); const end = Math.max(...times)
  return { series: series.map((item, index) => ({ ...item, color: colors[index % colors.length], path: item.points.map(([time, value], pointIndex) => `${pointIndex ? 'L' : 'M'}${(38 + (time - start) / (end - start || 1) * 922).toFixed(2)},${(18 + (1 - (value - min) / (max - min || 1)) * 188).toFixed(2)}`).join(' ') })), min, max, start, end }
}
function formatCPU(value) { const n = Number(value); return !Number.isFinite(n) ? '--' : n < 1 ? `${(n * 1000).toFixed(n * 1000 >= 10 ? 0 : 1)} m` : at('coreUnit', { value: n.toFixed(2) }) }
function formatBytes(value) { let n = Number(value); if (!Number.isFinite(n)) return '--'; const units = ['B', 'KiB', 'MiB', 'GiB']; let i = 0; while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ } return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)} ${units[i]}` }
function formatRate(value) { return `${formatBytes(value)}/s` }
function formatPercent(value) { const n = Number(value); return Number.isFinite(n) ? `${n.toFixed(n >= 10 ? 0 : 1)}%` : '--' }
function formatMetric(key, value) { if (key === 'cpu') return formatCPU(value); if (key === 'network') return formatRate(value); if (key === 'wss') return formatPercent(value); return formatBytes(value) }
function timeText(timestamp) { return timestamp ? new Date(timestamp * 1000).toLocaleTimeString(currentLocale.value === 'en-US' ? 'en-US' : 'ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false }) : '--' }
function tickValue(chart, index, formatter) { return formatter(chart.max - (index - 1) * (chart.max - chart.min) / 3) }
function chartY(chart, value) { return 18 + (1 - (value - chart.min) / (chart.max - chart.min || 1)) * 188 }
function closestPoint(points, timestamp) { return points.reduce((closest, point) => !closest || Math.abs(point[0] - timestamp) < Math.abs(closest[0] - timestamp) ? point : closest, null) }
function updateHover(kind, event, chart) {
  if (!chart.series.length) return
  const rect = event.currentTarget.getBoundingClientRect(); const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)); const timestamp = chart.start + (chart.end - chart.start) * ratio
  const target = { cpu: cpuHover, memory: memoryHover, wss: wssHover, network: networkHover }[kind]
  target.value = { visible: true, x: Math.max(12, Math.min(88, ratio * 100)), chartX: 38 + ratio * 922, time: timestamp, values: chart.series.map((series) => { const point = closestPoint(series.points, timestamp); return { name: series.name, color: series.color, value: point?.[1], y: point ? chartY(chart, point[1]) : 0 } }) }
}
function clearHover(kind) { const target = { cpu: cpuHover, memory: memoryHover, wss: wssHover, network: networkHover }[kind]; target.value = { visible: false } }
async function load() {
  const isServiceView = Boolean(props.serviceId)
  if ((!isServiceView && (!props.clusterId || !props.namespace)) || !props.workloadType || !props.workloadName) return
  loading.value = true; status.value = 'loading'; cpuSeries.value = []; memorySeries.value = []; wssSeries.value = []; networkInSeries.value = []; networkOutSeries.value = []
  try {
    const data = isServiceView ? await queryAssetServiceWorkloadMetrics({ serviceId: props.serviceId, workloadType: props.workloadType, workloadName: props.workloadName, range: range.value }) : await queryK8sWorkloadMetrics(props.clusterId, props.namespace, props.workloadType, props.workloadName, range.value)
    cpuSeries.value = normalizeSeries(data?.metrics?.cpu?.series); memorySeries.value = normalizeSeries(data?.metrics?.memory?.series); wssSeries.value = normalizeSeries(data?.metrics?.wss?.series); networkInSeries.value = normalizeSeries(data?.metrics?.networkIn?.series).map((item) => ({ ...item, direction: 'in' })); networkOutSeries.value = normalizeSeries(data?.metrics?.networkOut?.series).map((item) => ({ ...item, direction: 'out' })); status.value = data?.status || 'unavailable'
  } catch (error) { console.warn('Failed to load service pod metrics', error); status.value = 'unavailable' } finally { loading.value = false }
}
watch(() => [props.serviceId, props.clusterId, props.namespace, props.workloadType, props.workloadName, range.value], load, { immediate: true })
</script>

<template>
  <section class="service-pod-monitor" :aria-label="at('podMonitoring')">
    <header><div><h3>{{ at('podMonitoring') }}</h3><p>{{ at('podMonitoringDesc') }}</p></div><div class="actions"><el-select v-model="range" size="small"><el-option :label="at('lastHour')" value="1h" /><el-option :label="at('last6Hours')" value="6h" /><el-option :label="at('last24Hours')" value="24h" /></el-select><el-button size="small" :loading="loading" @click="load">{{ at('refresh') }}</el-button></div></header>
    <div v-loading="loading" class="monitor-body"><div v-if="hasData" class="monitor-grid">
      <article v-for="metric in metricCards" :key="metric.key" class="metric-card"><div class="metric-title"><b>{{ metric.title }}</b><span>{{ metric.unit }}</span></div><div class="chart-wrap"><svg viewBox="0 0 1000 230" preserveAspectRatio="none" role="img" :aria-label="at('trend', { title: metric.title })" @mousemove="updateHover(metric.key, $event, metric.chart)" @mouseleave="clearHover(metric.key)"><line v-for="index in 4" :key="index" x1="38" x2="960" :y1="18 + (index - 1) * 62.7" :y2="18 + (index - 1) * 62.7" /><text v-for="index in 4" :key="`${metric.key}-${index}`" x="0" :y="22 + (index - 1) * 62.7">{{ tickValue(metric.chart, index, (value) => formatMetric(metric.key, value)) }}</text><path v-for="series in metric.chart.series" :key="series.name" :d="series.path" :stroke="series.color" /><template v-if="metric.hover.visible"><line class="hover-line" :x1="metric.hover.chartX" :x2="metric.hover.chartX" y1="18" y2="206" /><circle v-for="item in metric.hover.values" :key="item.name" :cx="metric.hover.chartX" :cy="item.y" r="3.5" :fill="item.color" /></template></svg><div v-if="metric.hover.visible" class="chart-tooltip" :style="{ left: `${metric.hover.x}%` }"><strong>{{ timeText(metric.hover.time) }}</strong><div v-for="item in metric.hover.values" :key="item.name"><i :style="{ background: item.color }" /><span>{{ item.name }}</span><b>{{ formatMetric(metric.key, item.value) }}</b></div></div></div><div class="axis"><span>{{ timeText(metric.chart.start) }}</span><span>{{ timeText(metric.chart.end) }}</span></div><div class="legend"><div v-for="series in metric.chart.series" :key="series.name"><i :style="{ background: series.color }" /><span>{{ series.name }}</span><b>{{ formatMetric(metric.key, series.latest) }}</b></div></div></article>
      </div><el-empty v-else :image-size="50" :description="status === 'not-configured' ? at('noMetricsDatasource') : at('noPodMetrics')" /></div>
  </section>
</template>

<style scoped>
.service-pod-monitor{margin-top:18px;padding:20px;border:1px solid #dce7f4;border-radius:14px;background:linear-gradient(135deg,#fff,#f7faff)}.service-pod-monitor header,.actions,.metric-title,.axis{display:flex;align-items:center}.service-pod-monitor header{justify-content:space-between;gap:16px}.service-pod-monitor h3{margin:0;color:#153969;font-size:18px}.service-pod-monitor p{margin:6px 0 0;color:#7486a1;font-size:12px}.actions{gap:8px}.actions .el-select{width:118px}.monitor-body{min-height:250px}.monitor-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin-top:16px}.metric-card{padding:14px;border:1px solid #e3eaf4;border-radius:10px;background:#fff}.metric-title{justify-content:space-between;color:#183962}.metric-title span{color:#8796ab;font-size:12px}.chart-wrap{position:relative}.metric-card svg{display:block;width:100%;height:210px;margin-top:12px;overflow:visible;cursor:crosshair}.metric-card line{stroke:#e8edf4;stroke-width:1;vector-effect:non-scaling-stroke}.metric-card .hover-line{stroke:#7d8fa9;stroke-dasharray:3 3}.metric-card text{fill:#8996aa;font-size:11px}.metric-card path{fill:none;stroke-width:2;vector-effect:non-scaling-stroke}.chart-tooltip{position:absolute;z-index:2;top:22px;min-width:164px;padding:9px 10px;border:1px solid #d6e1ef;border-radius:7px;background:rgba(255,255,255,.96);box-shadow:0 7px 18px rgba(42,69,104,.18);transform:translateX(-50%);pointer-events:none}.chart-tooltip strong{display:block;margin-bottom:5px;color:#294b79;font-size:12px}.chart-tooltip div{display:grid;grid-template-columns:9px minmax(0,1fr) auto;gap:6px;align-items:center;padding:2px 0;color:#63758f;font-size:11px}.chart-tooltip i{width:7px;height:7px;border-radius:50%}.chart-tooltip span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.chart-tooltip b{color:#294b79;font-weight:600}.axis{justify-content:space-between;padding-left:38px;color:#8996aa;font-size:11px}.legend{max-height:124px;margin-top:12px;overflow:auto;border-top:1px solid #edf1f6}.legend>div{display:grid;grid-template-columns:10px minmax(0,1fr) auto;gap:8px;align-items:center;padding:6px 0;border-bottom:1px solid #f0f3f7;color:#62748f;font-size:12px}.legend i{width:8px;height:8px;border-radius:50%}.legend span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.legend b{color:#294b79;font-weight:600}@media(max-width:1100px){.monitor-grid{grid-template-columns:1fr}}
</style>
