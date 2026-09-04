<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { queryAssetHostMetrics } from '../../api/asset'

const props = defineProps({ hostId: { type: Number, required: true } })
const activeRange = ref('1h')
const customRange = ref([])
const loading = ref(false)
const data = ref({ metrics: {}, status: 'unavailable' })
const ranges = [
  ['1h', '1시간'], ['3h', '3시간'], ['6h', '6시간'], ['12h', '12시간'],
  ['1d', '1일'], ['3d', '3일'], ['7d', '7일'], ['14d', '14일'], ['custom', '직접 지정']
]
const metricCards = [
  ['cpu', 'CPU 사용률', '#4f7cff'], ['memory', '메모리 사용률', '#28b889'],
  ['disk', '디스크 사용률', '#f0a04a'], ['io', 'IO 부하', '#8c67e8']
]
const hasMetrics = computed(() => metricCards.some(([key]) => (data.value.metrics?.[key]?.points || []).length))
const queryError = computed(() => Object.values(data.value.errors || {})[0] || '')

function rangeLabel(key) { return ranges.find((item) => item[0] === key)?.[1] || key }
function displayValue(value) { return Number.isFinite(Number(value)) ? `${Number(value).toFixed(1)}%` : '-' }
function timeLabel(timestamp) { return new Date(timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
function validPoints(points) { return (points || []).filter((item) => Number.isFinite(Number(item.value))) }
function scaleFor(points) {
  const values = validPoints(points).map((item) => Number(item.value))
  if (!values.length) return { min: 0, max: 100, span: 100 }
  const rawMin = Math.min(...values); const rawMax = Math.max(...values)
  const padding = Math.max((rawMax - rawMin) * 0.16, rawMax * 0.04, 0.4)
  const min = Math.max(0, rawMin - padding); const max = Math.min(100, rawMax + padding)
  return { min, max: Math.max(max, min + 1), span: Math.max(max - min, 1) }
}
function chartCoordinate(points, index) {
  const items = validPoints(points); const scale = scaleFor(items); const point = items[index]
  if (!point) return { x: 48, y: 174 }
  return { x: 48 + (index / Math.max(items.length - 1, 1)) * 530, y: 174 - ((Number(point.value) - scale.min) / scale.span) * 132 }
}
function chartPoints(points) { return validPoints(points).map((_, index) => { const point = chartCoordinate(points, index); return `${point.x.toFixed(1)},${point.y.toFixed(1)}` }).join(' ') }
function chartArea(points) { const line = chartPoints(points); return line ? `M 48,174 L ${line.replaceAll(' ', ' L ')} L 578,174 Z` : '' }
function yTicks(points) { const scale = scaleFor(points); return [scale.max, scale.min + scale.span / 2, scale.min].map((value, index) => ({ value, y: 42 + index * 66 })) }
function xTicks(points) { const items = validPoints(points); return [0, 0.33, 0.66, 1].map((ratio) => { const index = Math.round((items.length - 1) * ratio); return { x: 48 + ratio * 530, label: items[index] ? timeLabel(items[index].timestamp) : '-' } }) }
function lastPoint(points) { const items = validPoints(points); return items.length ? chartCoordinate(items, items.length - 1) : null }
function latestTime(points) { return points?.length ? timeLabel(points[points.length - 1].timestamp) : '-' }
async function loadMetrics() {
  if (activeRange.value === 'custom' && customRange.value.length !== 2) return
  loading.value = true
  try {
    const params = { id: props.hostId, range: activeRange.value }
    if (activeRange.value === 'custom') { params.start = customRange.value[0]; params.end = customRange.value[1] }
    data.value = await queryAssetHostMetrics(params)
  } finally { loading.value = false }
}
function chooseRange(key) { activeRange.value = key; if (key !== 'custom') loadMetrics() }
watch(customRange, () => { if (activeRange.value === 'custom' && customRange.value.length === 2) loadMetrics() })
onMounted(loadMetrics)
</script>

<template>
  <section class="metrics-section" v-loading="loading">
    <header class="metrics-header">
      <div><h3>&#36816;&#34892;&#25351;&#26631;</h3><p>&#25968;&#25454;&#26469;&#33258;&#26412;&#22320; Prometheus / VictoriaMetrics&#12290;</p></div>
      <div class="range-tools">
        <el-button-group><el-button v-for="item in ranges" :key="item[0]" :type="activeRange === item[0] ? 'primary' : 'default'" @click="chooseRange(item[0])">{{ item[1] }}</el-button></el-button-group>
        <el-date-picker v-if="activeRange === 'custom'" v-model="customRange" type="datetimerange" value-format="x" range-separator="-" start-placeholder="&#24320;&#22987;" end-placeholder="&#32467;&#26463;" style="width: 330px" />
      </div>
    </header>
    <div v-if="data.status === 'not_configured'" class="metrics-empty">&#26410;&#37197;&#32622; Prometheus / VictoriaMetrics &#25351;&#26631;&#25968;&#25454;&#28304;&#12290;</div>
    <div v-else-if="data.status === 'query_failed'" class="metrics-empty">&#25351;&#26631;&#25968;&#25454;&#28304;&#26597;&#35810;&#22833;&#36133;&#65306;{{ queryError }}</div>
    <div v-else-if="!hasMetrics" class="metrics-empty">&#24403;&#21069;&#26102;&#38388;&#33539;&#22260;&#27809;&#26377;&#21305;&#37197;&#21040;&#35813;&#20027;&#26426;&#30340;&#25351;&#26631;&#25968;&#25454;&#12290;</div>
    <div v-else class="metrics-grid">
      <article v-for="item in metricCards" :key="item[0]" class="metric-card">
        <header><div><h4>{{ item[1] }}</h4><strong :style="{ color: item[2] }">{{ displayValue(data.metrics?.[item[0]]?.latest) }}</strong></div><small>{{ rangeLabel(activeRange) }} · {{ latestTime(data.metrics?.[item[0]]?.points) }}</small></header>
        <svg viewBox="0 0 626 214" preserveAspectRatio="none" role="img">
          <defs><linearGradient :id="`metric-area-${item[0]}`" x1="0" x2="0" y1="0" y2="1"><stop :stop-color="item[2]" stop-opacity=".24"/><stop :stop-color="item[2]" stop-opacity=".015" offset="1"/></linearGradient></defs>
          <g v-for="tick in yTicks(data.metrics?.[item[0]]?.points)" :key="tick.y"><line x1="48" :y1="tick.y" x2="578" :y2="tick.y"/><text x="40" :y="tick.y + 4" text-anchor="end">{{ tick.value.toFixed(1) }}%</text></g>
          <path :d="chartArea(data.metrics?.[item[0]]?.points)" :fill="`url(#metric-area-${item[0]})`"/>
          <polyline :points="chartPoints(data.metrics?.[item[0]]?.points)" :stroke="item[2]"/>
          <circle v-if="lastPoint(data.metrics?.[item[0]]?.points)" :cx="lastPoint(data.metrics?.[item[0]]?.points).x" :cy="lastPoint(data.metrics?.[item[0]]?.points).y" r="4" :fill="item[2]"/>
          <g v-for="tick in xTicks(data.metrics?.[item[0]]?.points)" :key="tick.x"><line :x1="tick.x" y1="174" :x2="tick.x" y2="178"/><text :x="tick.x" y="199" text-anchor="middle">{{ tick.label }}</text></g>
        </svg>
      </article>
    </div>
  </section>
</template>

<style scoped>
.metrics-section{padding:20px;border:1px solid #e3eaf5;border-radius:8px;background:#fff}.metrics-header{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:18px}.metrics-header h3{margin:0;color:#13294b;font-size:17px}.metrics-header p{margin:6px 0 0;color:#7b8ca8;font-size:13px}.range-tools{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.metrics-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.metric-card{padding:18px;border:1px solid #e7edf6;border-radius:12px;background:linear-gradient(180deg,#fff,#fcfdff)}.metric-card header{display:flex;align-items:end;justify-content:space-between;gap:12px}.metric-card h4{margin:0;color:#213856;font-size:15px}.metric-card strong{display:block;margin-top:8px;font-size:25px;line-height:1}.metric-card small{color:#8292aa;font-size:12px}.metric-card svg{display:block;width:100%;height:204px;margin-top:10px;overflow:visible}.metric-card line{stroke:#e8eef7;stroke-width:1}.metric-card text{fill:#8493a9;font-size:10px}.metric-card polyline{fill:none;stroke-width:2.8;stroke-linecap:round;stroke-linejoin:round}.metric-card circle{stroke:#fff;stroke-width:2}.metrics-empty{padding:42px 20px;border:1px dashed #d9e3f1;border-radius:10px;background:#fafcff;color:#7e8ea7;text-align:center}@media(max-width:1000px){.metrics-header{flex-direction:column}.metrics-grid{grid-template-columns:1fr}}
</style>
