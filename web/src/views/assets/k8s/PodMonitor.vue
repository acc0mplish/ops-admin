<script setup>
import { computed, ref, watch } from 'vue'
import { queryK8sPodMetrics } from '../../../api/k8s'

const props = defineProps({
  pod: {
    type: Object,
    required: true
  },
  clusterId: {
    type: Number,
    required: true
  }
})

const range = ref('1h')
const loading = ref(false)
const status = ref('idle')
const cpuPoints = ref([])
const memoryPoints = ref([])

const rangeSeconds = { '1h': 3600, '6h': 21600, '24h': 86400 }
const cpuChart = computed(() => buildChart(cpuPoints.value))
const memoryChart = computed(() => buildChart(memoryPoints.value))
const latestCPU = computed(() => cpuPoints.value.at(-1)?.[1])
const latestMemory = computed(() => memoryPoints.value.at(-1)?.[1])
const hasData = computed(() => cpuPoints.value.length || memoryPoints.value.length)

function toPoints(series) {
  return (series?.points || [])
    .map((point) => [Number(point.timestamp) / 1000, Number(point.value)])
    .filter(([time, value]) => Number.isFinite(time) && Number.isFinite(value))
    .sort((left, right) => left[0] - right[0])
}

function buildChart(points) {
  if (!points.length) return { path: '', min: 0, max: 1, start: 0, end: 0 }
  const values = points.map((item) => item[1])
  const min = Math.min(...values)
  const rawMax = Math.max(...values)
  const max = rawMax === min ? min + Math.max(1, Math.abs(min) * 0.08) : rawMax
  const start = points[0][0]
  const end = points.at(-1)[0]
  const path = points.map(([time, value], index) => {
    const x = 10 + (time - start) / (end - start || 1) * 480
    const y = 10 + (1 - (value - min) / (max - min || 1)) * 120
    return `${index ? 'L' : 'M'}${x.toFixed(2)},${y.toFixed(2)}`
  }).join(' ')
  return { path, min, max, start, end }
}

function formatCPU(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '--'
  return number < 1 ? `${(number * 1000).toFixed(number * 1000 >= 10 ? 0 : 1)} m` : `${number.toFixed(2)} 核`
}

function formatBytes(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '--'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let index = 0
  let current = number
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

function timeText(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : '--'
}

async function loadMetrics() {
  const namespace = String(props.pod?.namespace || '').trim()
  const podName = String(props.pod?.name || '').trim()
  if (!namespace || !podName) return
  loading.value = true
  status.value = 'loading'
  cpuPoints.value = []
  memoryPoints.value = []
  try {
    const data = await queryK8sPodMetrics(props.clusterId, namespace, podName, range.value)
    cpuPoints.value = toPoints(data?.metrics?.cpu)
    memoryPoints.value = toPoints(data?.metrics?.memory)
    status.value = data?.status || (hasData.value ? 'available' : 'unavailable')
  } catch (error) {
    console.warn('Failed to load pod metrics', error)
    status.value = 'unavailable'
  } finally {
    loading.value = false
  }
}

watch(() => [props.clusterId, props.pod?.namespace, props.pod?.name, range.value], loadMetrics, { immediate: true })
</script>

<template>
  <section class="pod-monitor-section" aria-label="Pod 监控">
    <div class="pod-monitor-head">
      <div>
        <strong>Pod 监控</strong>
        <span>CPU / 内存使用趋势</span>
      </div>
      <div class="pod-monitor-actions">
        <el-select v-model="range" size="small" aria-label="监控时间范围">
          <el-option label="最近 1 小时" value="1h" />
          <el-option label="最近 6 小时" value="6h" />
          <el-option label="最近 24 小时" value="24h" />
        </el-select>
        <el-button size="small" :loading="loading" @click="loadMetrics">刷新</el-button>
      </div>
    </div>

    <div v-loading="loading" class="pod-monitor-body">
      <div v-if="hasData" class="pod-monitor-charts">
        <article class="pod-monitor-card cpu">
          <div class="pod-monitor-card-head"><span>CPU</span><b>{{ formatCPU(latestCPU) }}</b></div>
          <svg viewBox="0 0 500 140" preserveAspectRatio="none" role="img" aria-label="Pod CPU 使用趋势">
            <line v-for="line in 4" :key="line" x1="10" x2="490" :y1="10 + (line - 1) * 40" :y2="10 + (line - 1) * 40" />
            <path :d="cpuChart.path" />
          </svg>
          <div class="pod-monitor-axis"><span>{{ timeText(cpuChart.start) }}</span><span>{{ timeText(cpuChart.end) }}</span></div>
        </article>
        <article class="pod-monitor-card memory">
          <div class="pod-monitor-card-head"><span>内存</span><b>{{ formatBytes(latestMemory) }}</b></div>
          <svg viewBox="0 0 500 140" preserveAspectRatio="none" role="img" aria-label="Pod 内存使用趋势">
            <line v-for="line in 4" :key="line" x1="10" x2="490" :y1="10 + (line - 1) * 40" :y2="10 + (line - 1) * 40" />
            <path :d="memoryChart.path" />
          </svg>
          <div class="pod-monitor-axis"><span>{{ timeText(memoryChart.start) }}</span><span>{{ timeText(memoryChart.end) }}</span></div>
        </article>
      </div>
      <el-empty v-else :image-size="52" :description="status === 'not-configured' ? '未配置 Prometheus 或 VictoriaMetrics 数据源' : '暂无 Pod 监控数据'" />
    </div>
  </section>
</template>

<style scoped>
.pod-monitor-section { padding: 16px; border: 1px solid #dfe8f7; border-radius: 10px; background: linear-gradient(135deg, #fbfdff, #f5f8ff); }
.pod-monitor-head, .pod-monitor-actions, .pod-monitor-card-head, .pod-monitor-axis { display: flex; align-items: center; }
.pod-monitor-head { justify-content: space-between; gap: 16px; }
.pod-monitor-head > div:first-child { display: flex; align-items: baseline; gap: 10px; }
.pod-monitor-head strong { color: #173d72; font-size: 16px; }
.pod-monitor-head span { color: #7a8aa3; font-size: 12px; }
.pod-monitor-actions { gap: 8px; }
.pod-monitor-actions .el-select { width: 118px; }
.pod-monitor-body { min-height: 180px; }
.pod-monitor-charts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.pod-monitor-card { padding: 12px; border: 1px solid #e4eaf5; border-radius: 8px; background: #fff; }
.pod-monitor-card-head { justify-content: space-between; color: #60728c; font-size: 13px; }
.pod-monitor-card-head b { color: #1e3b67; font-size: 16px; }
.pod-monitor-card svg { display: block; width: 100%; height: 132px; margin-top: 8px; }
.pod-monitor-card line { stroke: #e8eef7; stroke-width: 1; vector-effect: non-scaling-stroke; }
.pod-monitor-card path { fill: none; stroke-width: 2.4; vector-effect: non-scaling-stroke; }
.pod-monitor-card.cpu path { stroke: #3c77e8; }
.pod-monitor-card.memory path { stroke: #20a581; }
.pod-monitor-axis { justify-content: space-between; color: #8b98ac; font-size: 11px; }
@media (max-width: 900px) { .pod-monitor-charts { grid-template-columns: 1fr; } .pod-monitor-head { align-items: flex-start; flex-direction: column; } }
</style>
