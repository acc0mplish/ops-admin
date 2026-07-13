<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { queryMonitorDatasourceOptions, queryMonitorPrometheus, queryMonitorQueryHistoryList } from '../../api/monitor'

const loading = ref(false)
const datasourceOptions = ref([])
const datasourceId = ref()
const promql = ref('up')
const resultType = ref('')
const rows = ref([])
const metricTemplateId = ref('')
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyRows = ref([])
const historyTotal = ref(0)
const historyQuery = ref({ pageNum: 1, pageSize: 10, keyword: '', status: '' })

const metricTemplates = [
  { id: 'host-up', category: '主机资源', name: '主机采集状态', query: 'up{job=~"node.*|node-exporter"}', description: '逐台展示主机采集状态，保留 instance 等标签。' },
  { id: 'host-cpu', category: '主机资源', name: '主机 CPU 使用率', query: '100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)', description: '按实例展示近 5 分钟 CPU 使用率。' },
  { id: 'host-memory', category: '主机资源', name: '主机内存使用率', query: '(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100', description: '逐台展示主机内存使用率。' },
  { id: 'host-disk', category: '主机资源', name: '主机磁盘使用率', query: '100 - (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} * 100)', description: '按主机和挂载点展示磁盘使用率。' },
  { id: 'host-cpu-top', category: '主机资源', name: 'CPU 使用率 Top 10', query: 'topk(10, 100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100))', description: '按实例查看 CPU 使用率最高的十台主机。' },
  { id: 'host-memory-top', category: '主机资源', name: '内存使用率 Top 10', query: 'topk(10, (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100)', description: '按实例查看内存使用率最高的十台主机。' },
  { id: 'host-load-top', category: '主机资源', name: '系统负载 Top 10', query: 'topk(10, node_load1)', description: '按实例查看 1 分钟系统负载最高的十台主机。' },
  { id: 'host-network-in', category: '主机资源', name: '网络接收速率 Top 10', query: 'topk(10, sum by (instance) (rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|br.*"}[5m])))', description: '按实例查看近 5 分钟网络接收速率。' },
  { id: 'host-processes', category: '主机资源', name: '运行进程数', query: 'node_procs_running', description: '逐台展示主机正在运行的进程数。' },
  { id: 'k8s-pods', category: 'Kubernetes', name: 'Pod 基础信息', query: 'kube_pod_info', description: '逐个展示 Pod，保留 namespace、pod、node 等标签。' },
  { id: 'k8s-running-pods', category: 'Kubernetes', name: '运行中 Pod', query: 'kube_pod_status_phase{phase="Running"}', description: '逐个展示处于 Running 状态的 Pod。' },
  { id: 'k8s-abnormal-pods', category: 'Kubernetes', name: '异常 Pod', query: 'kube_pod_status_phase{phase=~"Failed|Unknown|Pending"}', description: '逐个展示 Failed、Unknown、Pending 状态的 Pod。' },
  { id: 'k8s-ready-nodes', category: 'Kubernetes', name: '节点就绪状态', query: 'kube_node_status_condition{condition="Ready",status="true"}', description: '逐节点展示 Ready 状态。' },
  { id: 'k8s-namespaces', category: 'Kubernetes', name: '命名空间', query: 'kube_namespace_created', description: '展示所有命名空间及创建时间序列。' },
  { id: 'k8s-workloads', category: 'Kubernetes', name: 'Deployment 工作负载', query: 'kube_deployment_created', description: '展示 Deployment 工作负载及其标签。' },
  { id: 'k8s-pod-distribution', category: 'Kubernetes', name: '命名空间 Pod 分布', query: 'sum by (namespace) (kube_pod_info)', description: '按命名空间统计 Pod 分布。' },
  { id: 'k8s-restarts', category: 'Kubernetes', name: 'Pod 重启次数 Top 10', query: 'topk(10, sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total[1h])))', description: '统计最近一小时重启次数最多的 Pod。' }
]

const selectedMetricTemplate = computed(() => metricTemplates.find((item) => item.id === metricTemplateId.value))

function metricText(metric) {
  return Object.entries(metric || {})
    .filter(([key]) => key !== '__name__')
    .map(([key, value]) => `${key}="${value}"`)
    .join(', ') || '无标签（汇总结果）'
}

function metricName(metric) {
  return metric?.__name__ || selectedMetricTemplate.value?.name || '汇总结果'
}

function formatTimestamp(timestamp) {
  const value = Number(timestamp)
  if (!Number.isFinite(value)) return timestamp || '-'
  return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false })
}

async function loadOptions() {
  const options = await queryMonitorDatasourceOptions()
  datasourceOptions.value = (options || []).filter((item) => ['prometheus', 'victoriametrics'].includes(item.type))
  datasourceId.value = datasourceOptions.value.find((item) => item.isDefault)?.id || datasourceOptions.value[0]?.id
}

function applyMetricTemplate(id) {
  const template = metricTemplates.find((item) => item.id === id)
  if (!template) return
  promql.value = template.query
  rows.value = []
  resultType.value = ''
}

async function executeQuery() {
  if (!datasourceId.value || !promql.value.trim()) {
    ElMessage.warning('请选择 Prometheus 或 VictoriaMetrics 数据源并输入 PromQL')
    return
  }
  loading.value = true
  try {
    const data = await queryMonitorPrometheus({ datasourceId: datasourceId.value, query: promql.value })
    resultType.value = data.resultType || ''
    rows.value = data.result || []
    if (historyVisible.value) await loadHistory()
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const data = await queryMonitorQueryHistoryList(historyQuery.value)
    historyRows.value = data.list || []
    historyTotal.value = data.total || 0
  } finally {
    historyLoading.value = false
  }
}

async function openHistory() {
  historyVisible.value = true
  await loadHistory()
}

function useHistory(row) {
  const datasource = datasourceOptions.value.find((item) => item.name === row.datasourceName)
  if (!datasource && !datasourceOptions.value.some((item) => item.id === row.datasourceId)) {
    ElMessage.warning('该记录来自非指标数据源，请在日志查询页面中使用')
    return
  }
  datasourceId.value = row.datasourceId || datasource?.id || datasourceId.value
  promql.value = row.query || promql.value
  historyVisible.value = false
}

onMounted(async () => {
  await loadOptions()
  if (!datasourceId.value) ElMessage.warning('请先在数据源管理中配置 Prometheus 或 VictoriaMetrics 数据源')
})
</script>

<template>
  <div class="monitor-query">
    <div class="query-top">
      <div>
        <h2>即时查询</h2>
        <p>仅支持 Prometheus 与 VictoriaMetrics，输入 PromQL 直接查询指标、调试告警规则和排查现场。</p>
      </div>
      <div class="query-top-actions">
        <el-select v-model="datasourceId" filterable placeholder="选择数据源" style="width: 260px">
          <el-option v-for="item in datasourceOptions" :key="item.id" :label="`${item.name} (${item.type})`" :value="item.id" />
        </el-select>
        <el-button @click="openHistory">查询历史</el-button>
      </div>
    </div>

    <div class="query-editor">
      <div class="template-toolbar">
        <span>内置指标</span>
        <el-select v-model="metricTemplateId" clearable filterable placeholder="选择主机或 Kubernetes 指标" style="width: 310px" @change="applyMetricTemplate">
          <el-option v-for="item in metricTemplates" :key="item.id" :label="`${item.category} / ${item.name}`" :value="item.id" />
        </el-select>
        <span class="template-description">{{ selectedMetricTemplate?.description || '从巡检大屏指标中选择一个查询模板。' }}</span>
      </div>
      <el-input v-model="promql" type="textarea" :rows="5" placeholder="例如：up 或 sum(rate(http_requests_total[5m]))" />
      <div class="query-actions">
        <el-button type="primary" :loading="loading" @click="executeQuery">执行查询</el-button>
        <span>Result Type: {{ resultType || '-' }}</span>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="指标" min-width="220" show-overflow-tooltip>
        <template #default="{ row }"><code class="metric-name">{{ metricName(row.metric) }}</code></template>
      </el-table-column>
      <el-table-column label="标签" min-width="420" show-overflow-tooltip>
        <template #default="{ row }"><span class="metric-labels">{{ metricText(row.metric) }}</span></template>
      </el-table-column>
      <el-table-column label="当前值" width="180">
        <template #default="{ row }">{{ row.value?.[1] ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="采样时间" width="190">
        <template #default="{ row }">{{ formatTimestamp(row.value?.[0]) }}</template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="historyVisible" title="查询历史" size="640px">
      <div class="history-toolbar">
        <el-input v-model="historyQuery.keyword" clearable placeholder="搜索查询语句 / 数据源" @keyup.enter="loadHistory" />
        <el-select v-model="historyQuery.status" clearable placeholder="状态" style="width: 120px">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-button type="primary" @click="loadHistory">搜索</el-button>
      </div>
      <el-table v-loading="historyLoading" :data="historyRows" border>
        <el-table-column prop="datasourceName" label="数据源" width="140" />
        <el-table-column prop="queryType" label="类型" width="120" />
        <el-table-column prop="query" label="查询语句" min-width="240" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="90" />
        <el-table-column prop="createTime" label="时间" width="170" />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="useHistory(row)">使用</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="historyQuery.pageNum" v-model:page-size="historyQuery.pageSize" :total="historyTotal" layout="total, prev, pager, next" @current-change="loadHistory" @size-change="loadHistory" />
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.monitor-query { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.query-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.query-top-actions { display: flex; gap: 10px; align-items: center; }
.query-top h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.query-top p { margin: 0; color: #7282a0; }
.query-editor { border: 1px solid #dfe7f2; border-radius: 12px; overflow: hidden; }
.template-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; background: #f7f9fd; border-bottom: 1px solid #e5ecf6; color: #5b6c88; font-size: 13px; }
.template-toolbar > span:first-child { color: #263d65; font-weight: 600; white-space: nowrap; }
.template-description { flex: 1; color: #8491a9; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.query-editor :deep(.el-textarea__inner) { border: 0; border-radius: 0; font-family: Consolas, Monaco, monospace; font-size: 14px; }
.metric-name { color: #315dcf; font-size: 13px; }
.metric-labels { color: #52637f; font-family: Consolas, Monaco, monospace; font-size: 12px; }
.query-actions { display: flex; justify-content: space-between; align-items: center; padding: 12px; background: #f7f9fd; border-top: 1px solid #e5ecf6; color: #7282a0; }
.history-toolbar { display: flex; gap: 10px; margin-bottom: 14px; }
.pager { display: flex; justify-content: flex-end; margin-top: 14px; }
</style>
