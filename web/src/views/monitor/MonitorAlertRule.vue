<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifyRuleOptions } from '../../api/ops'
import {
  batchUpdateMonitorAlertRules,
  deleteMonitorAlertRule,
  monitorAlertRuleInfo,
  queryMonitorAlertRuleList,
  queryMonitorDatasourceOptions,
  runMonitorAlertRule,
  saveMonitorAlertRule,
  updateMonitorAlertRuleStatus
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const copyMode = ref(false)
const templateDialogVisible = ref(false)
const templateType = ref('metric')
const batchNotifyVisible = ref(false)
const batchNotifySaving = ref(false)
const batchNotifyRuleId = ref()
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const datasourceOptions = ref([])
const notifyRuleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', severity: '', alertType: '' })
const form = reactive({
  id: undefined,
  name: '',
  alertType: 'metric',
  datasourceScope: 'specific',
  datasourceId: undefined,
  queryText: '',
  logIndex: '_all',
  logTimeRangeSeconds: 300,
  comparator: '>',
  threshold: 0,
  forSeconds: 0,
  evalIntervalSeconds: 60,
  notifyRepeatIntervalMinutes: 30,
  maxNotifyCount: 0,
  severity: 'P2',
  labelsJson: '{}',
  annotationsJson: '{}',
  notifyEnabled: false,
  notifyRuleId: undefined,
  notifyRecoveryEnabled: true,
  status: 1,
  description: ''
})

const metricDatasourceOptions = computed(() => datasourceOptions.value.filter((item) => ['prometheus', 'victoriametrics'].includes(item.type)))
const logDatasourceOptions = computed(() => datasourceOptions.value.filter((item) => item.type === 'elasticsearch'))
const availableDatasourceOptions = computed(() => (form.alertType === 'log' ? logDatasourceOptions.value : metricDatasourceOptions.value))
const alertTypeLabel = computed(() => (form.alertType === 'log' ? '日志告警' : '监控告警'))
const dialogTitle = computed(() => (copyMode.value ? '复制告警规则' : (isEdit.value ? '编辑告警规则' : '新增告警规则')))

const commonRuleTemplates = [
  { id: 'metric-target-down', type: 'metric', name: '采集目标不可达', queryText: 'up == 0', comparator: '>', threshold: 0, forSeconds: 120, evalIntervalSeconds: 60, severity: 'P1', description: 'Prometheus 采集目标连续不可达。' },
  { id: 'metric-node-cpu', type: 'metric', name: '主机 CPU 使用率过高', queryText: '100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)', comparator: '>', threshold: 90, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '主机 CPU 使用率连续 5 分钟超过 90%。' },
  { id: 'metric-node-memory', type: 'metric', name: '主机内存使用率过高', queryText: '(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100', comparator: '>', threshold: 90, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '主机可用内存不足。' },
  { id: 'metric-node-disk', type: 'metric', name: '主机磁盘使用率过高', queryText: '100 - (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} * 100)', comparator: '>', threshold: 85, forSeconds: 600, evalIntervalSeconds: 60, severity: 'P2', description: '磁盘分区使用率超过 85%。' },
  { id: 'metric-node-load', type: 'metric', name: '主机系统负载过高', queryText: 'node_load1 / count by (instance) (node_cpu_seconds_total{mode="idle"})', comparator: '>', threshold: 1.5, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '每核平均系统负载持续偏高。' },
  { id: 'metric-pod-abnormal', type: 'metric', name: 'Kubernetes 异常 Pod', queryText: 'sum by (namespace, pod, phase) (kube_pod_status_phase{phase=~"Failed|Unknown|Pending"})', comparator: '>', threshold: 0, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: 'Pod 处于 Failed、Unknown 或 Pending 状态。' },
  { id: 'metric-pod-restarts', type: 'metric', name: 'Kubernetes Pod 重启频繁', queryText: 'sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total[10m]))', comparator: '>', threshold: 3, forSeconds: 0, evalIntervalSeconds: 60, severity: 'P2', description: 'Pod 在 10 分钟内重启超过 3 次。' },
  { id: 'metric-node-not-ready', type: 'metric', name: 'Kubernetes 节点未就绪', queryText: 'kube_node_status_condition{condition="Ready",status="true"} == 0', comparator: '>', threshold: 0, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: 'Kubernetes 节点连续 5 分钟未就绪。' },
  { id: 'metric-deployment-unavailable', type: 'metric', name: 'Deployment 副本不可用', queryText: 'kube_deployment_status_replicas_unavailable', comparator: '>', threshold: 0, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: 'Deployment 存在不可用副本。' },
  { id: 'metric-pvc-pending', type: 'metric', name: 'PVC 未绑定', queryText: 'kube_persistentvolumeclaim_status_phase{phase="Pending"}', comparator: '>', threshold: 0, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '持久卷声明持续处于 Pending 状态。' },
  { id: 'log-error', type: 'log', name: '应用 ERROR 日志激增', queryText: 'level:ERROR', logIndex: 'logs-*', comparator: '>', threshold: 20, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '近 5 分钟 ERROR 日志超过 20 条。' },
  { id: 'log-fatal', type: 'log', name: '应用 FATAL 日志', queryText: 'level:FATAL', logIndex: 'logs-*', comparator: '>', threshold: 0, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P0', description: '发现任意 FATAL 日志立即告警。' },
  { id: 'log-exception', type: 'log', name: '未捕获异常日志', queryText: 'message:(Exception OR Error OR Throwable)', logIndex: 'logs-*', comparator: '>', threshold: 10, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '近 5 分钟发生较多异常堆栈。' },
  { id: 'log-auth-failed', type: 'log', name: '认证失败异常', queryText: 'message:(login failed OR authentication failed OR unauthorized)', logIndex: 'logs-*', comparator: '>', threshold: 10, logTimeRangeSeconds: 600, evalIntervalSeconds: 60, severity: 'P2', description: '近 10 分钟认证失败次数超过阈值。' },
  { id: 'log-db-error', type: 'log', name: '数据库连接异常', queryText: 'message:(SQLException OR Connection refused OR connection pool)', logIndex: 'logs-*', comparator: '>', threshold: 5, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: '发现数据库连接或连接池异常。' },
  { id: 'log-timeout', type: 'log', name: '服务超时日志激增', queryText: 'message:(timeout OR timed out OR TimeoutException)', logIndex: 'logs-*', comparator: '>', threshold: 20, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', description: '近 5 分钟服务超时日志异常增多。' },
  { id: 'log-oom', type: 'log', name: '内存溢出日志', queryText: 'message:(OutOfMemoryError OR Java heap space OR killed process)', logIndex: 'logs-*', comparator: '>', threshold: 0, logTimeRangeSeconds: 600, evalIntervalSeconds: 60, severity: 'P0', description: '发现内存溢出或进程被系统回收。' },
  { id: 'log-http-5xx', type: 'log', name: 'HTTP 5xx 响应激增', queryText: 'message:(HTTP 500 OR HTTP 502 OR HTTP 503 OR HTTP 504)', logIndex: 'logs-*', comparator: '>', threshold: 20, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: '近 5 分钟服务端错误响应超过阈值。' },
  { id: 'log-k8s-crashloop', type: 'log', name: '容器崩溃重启日志', queryText: 'message:(CrashLoopBackOff OR Back-off restarting failed container)', logIndex: 'logs-*', comparator: '>', threshold: 0, logTimeRangeSeconds: 600, evalIntervalSeconds: 60, severity: 'P1', description: '发现容器 CrashLoopBackOff 相关日志。' },
  { id: 'log-payment-failed', type: 'log', name: '业务交易失败日志', queryText: 'message:(payment failed OR order failed OR transaction failed)', logIndex: 'logs-*', comparator: '>', threshold: 5, logTimeRangeSeconds: 300, evalIntervalSeconds: 60, severity: 'P1', description: '近 5 分钟关键业务交易失败超过阈值。' }
]

const visibleRuleTemplates = computed(() => commonRuleTemplates.filter((item) => item.type === templateType.value))

function formatNotifyInterval(seconds) {
  const value = Number(seconds) || 1800
  if (value % 86400 === 0) return `${value / 86400} 天`
  if (value % 3600 === 0) return `${value / 3600} 小时`
  if (value % 60 === 0) return `${value / 60} 分钟`
  return `${value} 秒`
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    alertType: 'metric',
    datasourceScope: 'specific',
    datasourceId: metricDatasourceOptions.value[0]?.id,
    queryText: '',
    logIndex: '_all',
    logTimeRangeSeconds: 300,
    comparator: '>',
    threshold: 0,
    forSeconds: 0,
    evalIntervalSeconds: 60,
    notifyRepeatIntervalMinutes: 30,
    maxNotifyCount: 0,
    severity: 'P2',
    labelsJson: '{}',
    annotationsJson: '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    status: 1,
    description: ''
  })
}

function applyDatasourceType() {
  form.datasourceId = form.datasourceScope === 'specific' ? availableDatasourceOptions.value[0]?.id : undefined
  if (form.alertType === 'log' && !form.queryText.trim()) form.queryText = 'level:ERROR OR level:FATAL'
}

function applyDatasourceScope() {
  form.datasourceId = form.datasourceScope === 'specific' ? availableDatasourceOptions.value[0]?.id : undefined
}

async function loadOptions() {
  const [datasources, notifyRules] = await Promise.all([
    queryMonitorDatasourceOptions(),
    queryNotifyRuleOptions({ scope: 'monitor' })
  ])
  datasourceOptions.value = datasources || []
  notifyRuleOptions.value = notifyRules || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  copyMode.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  copyMode.value = false
  const data = await monitorAlertRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    alertType: data.alertType || 'metric',
    datasourceScope: data.datasourceScope || 'specific',
    queryText: data.promql || '',
    logIndex: data.logIndex || '_all',
    logTimeRangeSeconds: data.logTimeRangeSeconds || 300,
    notifyRepeatIntervalMinutes: Math.max(1, Math.round((data.notifyRepeatIntervalSeconds || 1800) / 60)),
    maxNotifyCount: data.maxNotifyCount || 0,
    labelsJson: data.labelsJson || '{}',
    annotationsJson: data.annotationsJson || '{}'
  })
  dialogVisible.value = true
}

async function openCopy(row) {
  const data = await monitorAlertRuleInfo(row.id)
  isEdit.value = false
  copyMode.value = true
  Object.assign(form, {
    ...data,
    id: undefined,
    name: `${data.name} - 副本`,
    alertType: data.alertType || 'metric',
    datasourceScope: data.datasourceScope || 'specific',
    queryText: data.promql || '',
    logIndex: data.logIndex || '_all',
    logTimeRangeSeconds: data.logTimeRangeSeconds || 300,
    notifyRepeatIntervalMinutes: Math.max(1, Math.round((data.notifyRepeatIntervalSeconds || 1800) / 60)),
    maxNotifyCount: data.maxNotifyCount || 0,
    labelsJson: data.labelsJson || '{}',
    annotationsJson: data.annotationsJson || '{}'
  })
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateType.value = form.alertType || 'metric'
  templateDialogVisible.value = true
}

function applyRuleTemplate(template) {
  Object.assign(form, {
    id: undefined,
    name: template.name,
    alertType: template.type,
    datasourceScope: 'all',
    datasourceId: undefined,
    queryText: template.queryText,
    logIndex: template.logIndex || '_all',
    logTimeRangeSeconds: template.logTimeRangeSeconds || 300,
    comparator: template.comparator,
    threshold: template.threshold,
    forSeconds: template.forSeconds || 0,
    evalIntervalSeconds: template.evalIntervalSeconds,
    notifyRepeatIntervalMinutes: 30,
    maxNotifyCount: 0,
    severity: template.severity,
    labelsJson: '{}',
    annotationsJson: '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    status: 1,
    description: template.description
  })
  isEdit.value = false
  copyMode.value = false
  templateDialogVisible.value = false
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.queryText.trim() || (form.datasourceScope === 'specific' && !form.datasourceId)) {
    ElMessage.warning(`请填写规则名称、${form.datasourceScope === 'specific' ? '数据源和' : ''}${form.alertType === 'log' ? ' Elasticsearch 查询语句' : ' PromQL'}`)
    return
  }
  if (form.notifyEnabled && !form.notifyRuleId) {
    ElMessage.warning('请选择通知规则')
    return
  }
  saving.value = true
  try {
    await saveMonitorAlertRule({
      ...form,
      query: form.queryText,
      notifyRepeatIntervalSeconds: form.notifyRepeatIntervalMinutes * 60
    })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleStatus(row) {
  await updateMonitorAlertRuleStatus({ id: row.id, status: row.status === 1 ? 2 : 1 })
  ElMessage.success('状态已更新')
  await loadData()
}

async function handleRun(row) {
  await runMonitorAlertRule(row.id)
  ElMessage.success('已触发一次规则评估')
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除告警规则「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorAlertRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function handleSelectionChange(selection) {
  selectedRuleIds.value = selection.map((item) => item.id)
}

async function handleBatchStatus(status) {
  if (!selectedRuleIds.value.length) return
  const action = status === 1 ? 'enable' : 'disable'
  const label = status === 1 ? '启用' : '禁用'
  await ElMessageBox.confirm(`确认批量${label}已选中的 ${selectedRuleIds.value.length} 条告警规则吗？`, '批量操作确认', { type: 'warning' })
  await batchUpdateMonitorAlertRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(`已批量${label}`)
  selectedRuleIds.value = []
  await loadData()
}

function openBatchNotify() {
  if (!selectedRuleIds.value.length) return
  batchNotifyRuleId.value = undefined
  batchNotifyVisible.value = true
}

async function submitBatchNotify() {
  if (!batchNotifyRuleId.value) {
    ElMessage.warning('请选择通知规则')
    return
  }
  batchNotifySaving.value = true
  try {
    await batchUpdateMonitorAlertRules({ ids: selectedRuleIds.value, action: 'enable_notify', notifyRuleId: batchNotifyRuleId.value })
    ElMessage.success('已批量启用通知并绑定通知规则')
    batchNotifyVisible.value = false
    selectedRuleIds.value = []
    await loadData()
  } finally {
    batchNotifySaving.value = false
  }
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警规则</h2>
        <p>统一管理 PromQL 监控告警与 Elasticsearch 日志告警，支持按全部同类数据源或指定数据源进行评估。</p>
      </div>
      <div class="header-actions">
        <el-button @click="openTemplateDialog">导入常用规则</el-button>
        <el-button type="primary" @click="openCreate">新增规则</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索规则 / 查询语句" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.alertType" clearable placeholder="告警类型" style="width: 130px">
        <el-option label="监控告警" value="metric" />
        <el-option label="日志告警" value="log" />
      </el-select>
      <el-select v-model="query.severity" clearable placeholder="等级" style="width: 120px">
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <div v-if="selectedRuleIds.length" class="batch-toolbar">
      <span>已选择 <b>{{ selectedRuleIds.length }}</b> 条规则</span>
      <el-button size="small" type="success" @click="handleBatchStatus(1)">批量启用</el-button>
      <el-button size="small" type="warning" @click="handleBatchStatus(2)">批量禁用</el-button>
      <el-button size="small" type="primary" plain @click="openBatchNotify">批量启用通知</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" label="规则名称" min-width="180" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }"><el-tag :type="row.alertType === 'log' ? 'warning' : 'primary'" effect="light">{{ row.alertType === 'log' ? '日志告警' : '监控告警' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="datasourceName" label="数据源范围" width="180" />
      <el-table-column label="查询条件" min-width="340" show-overflow-tooltip>
        <template #default="{ row }">{{ row.promql }} {{ row.comparator }} {{ row.threshold }}</template>
      </el-table-column>
      <el-table-column prop="severity" label="等级" width="90" />
      <el-table-column prop="evalIntervalSeconds" label="评估间隔" width="110"><template #default="{ row }">{{ row.evalIntervalSeconds }} 秒</template></el-table-column>
      <el-table-column prop="lastEvalStatus" label="最近评估" width="120" />
      <el-table-column label="通知" width="90"><template #default="{ row }"><el-tag :type="row.notifyEnabled ? 'success' : 'info'">{{ row.notifyEnabled ? '启用' : '关闭' }}</el-tag></template></el-table-column>
      <el-table-column label="重复通知" width="120"><template #default="{ row }">{{ row.notifyEnabled ? formatNotifyInterval(row.notifyRepeatIntervalSeconds) : '-' }}</template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleRun(row)">运行</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="openCopy(row)">复制</el-button>
          <el-button link type="warning" @click="handleStatus(row)">{{ row.status === 1 ? '禁用' : '启用' }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="980px" top="5vh">
      <el-form label-width="120px">
        <el-form-item label="规则名称" required><el-input v-model="form.name" placeholder="例如：生产集群 Pod 异常 / 游戏服错误日志激增" /></el-form-item>
        <el-form-item label="快速导入"><el-button plain @click="openTemplateDialog">选择常用规则模板</el-button><span class="form-tip inline-tip">内置 10 条 PromQL 和 10 条 Elasticsearch 日志规则。</span></el-form-item>
        <el-form-item label="告警类型" required>
          <el-radio-group v-model="form.alertType" @change="applyDatasourceType">
            <el-radio-button label="metric">监控告警 / PromQL</el-radio-button>
            <el-radio-button label="log">日志告警 / Elasticsearch</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="数据源范围" required>
          <div class="datasource-scope">
            <el-radio-group v-model="form.datasourceScope" @change="applyDatasourceScope">
              <el-radio-button label="all">匹配所有{{ alertTypeLabel }}数据源</el-radio-button>
              <el-radio-button label="specific">指定数据源</el-radio-button>
            </el-radio-group>
            <el-select v-if="form.datasourceScope === 'specific'" v-model="form.datasourceId" filterable style="width: 360px" placeholder="选择数据源">
              <el-option v-for="item in availableDatasourceOptions" :key="item.id" :label="`${item.name} (${item.type})`" :value="item.id" />
            </el-select>
            <span v-else class="form-tip">将对所有已启用的{{ form.alertType === 'log' ? ' Elasticsearch' : ' Prometheus / VictoriaMetrics' }}数据源分别执行规则。</span>
          </div>
        </el-form-item>
        <el-form-item v-if="form.alertType === 'log'" label="日志索引" required><el-input v-model="form.logIndex" placeholder="例如：logs-*、.ds-app-log-*；使用 _all 搜索全部索引" /></el-form-item>
        <el-form-item :label="form.alertType === 'log' ? 'ES 查询语句' : 'PromQL'" required>
          <el-input v-model="form.queryText" type="textarea" :rows="4" :placeholder="form.alertType === 'log' ? '例如：level:ERROR AND kubernetes.pod_namespace:&quot;default&quot;' : '例如：sum(kube_pod_status_phase{phase=~&quot;Failed|Unknown&quot;})'" />
          <div class="form-tip">{{ form.alertType === 'log' ? '使用 Elasticsearch query_string（Lucene）语法，告警值为查询窗口内命中的日志条数。' : 'PromQL 返回的每条时间序列都会独立与阈值比较，标签会保留在告警事件中。' }}</div>
        </el-form-item>
        <section class="rule-parameters">
          <div class="parameter-card">
            <span>比较符</span>
            <el-select v-model="form.comparator"><el-option v-for="item in ['>','>=','<','<=','==','!=']" :key="item" :label="item" :value="item" /></el-select>
            <small>与阈值进行比较</small>
          </div>
          <div class="parameter-card">
            <span>{{ form.alertType === 'log' ? '命中阈值' : '阈值' }}</span>
            <el-input-number v-model="form.threshold" :precision="4" :step="form.alertType === 'log' ? 1 : 0.1" controls-position="right" />
            <small>{{ form.alertType === 'log' ? '窗口内匹配日志条数' : '单条时间序列当前值' }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ form.alertType === 'log' ? '查询窗口' : '持续时间' }}</span>
            <div class="parameter-number"><el-input-number v-if="form.alertType === 'log'" v-model="form.logTimeRangeSeconds" :min="60" :max="86400" controls-position="right" /><el-input-number v-else v-model="form.forSeconds" :min="0" :max="86400" controls-position="right" /><b>秒</b></div>
            <small>{{ form.alertType === 'log' ? '统计最近日志范围' : '连续命中后触发' }}</small>
          </div>
          <div class="parameter-card">
            <span>评估间隔</span>
            <div class="parameter-number"><el-input-number v-model="form.evalIntervalSeconds" :min="15" :max="3600" controls-position="right" /><b>秒</b></div>
            <small>系统执行规则的频率</small>
          </div>
          <div class="parameter-card notification-interval-card">
            <span>重复通知间隔</span>
            <div class="parameter-number"><el-input-number v-model="form.notifyRepeatIntervalMinutes" :min="1" :max="10080" controls-position="right" /><b>分钟</b></div>
            <small>告警持续触发时，间隔多久再次通知；默认 30 分钟</small>
          </div>
        </section>
        <el-form-item label="等级"><el-radio-group v-model="form.severity"><el-radio-button v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" /></el-radio-group></el-form-item>
        <el-form-item label="标签 JSON"><el-input v-model="form.labelsJson" type="textarea" :rows="2" placeholder='例如：{"team":"game"}' /></el-form-item>
        <el-form-item label="注解 JSON"><el-input v-model="form.annotationsJson" type="textarea" :rows="2" placeholder='例如：{"summary":"请及时处理"}' /></el-form-item>
        <el-form-item label="消息通知"><el-switch v-model="form.notifyEnabled" /></el-form-item>
        <el-form-item v-if="form.notifyEnabled" label="通知规则" required><el-select v-model="form.notifyRuleId" filterable style="width: 100%"><el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item>
        <el-form-item v-if="form.notifyEnabled" label="最大发送次数"><el-input-number v-model="form.maxNotifyCount" :min="0" :max="1000" controls-position="right" /><span class="unit">0 表示不限制次数，首次触发通知也计入次数。</span></el-form-item>
        <el-form-item v-if="form.notifyEnabled" label="恢复通知"><el-switch v-model="form.notifyRecoveryEnabled" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio :value="1">启用</el-radio><el-radio :value="2">禁用</el-radio></el-radio-group></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="补充影响范围、处置建议或值班说明" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" title="导入常用告警规则" width="900px" top="10vh">
      <div class="template-dialog-head">
        <div><strong>常用告警规则</strong><span>模板导入后仍可修改查询语句、阈值和通知策略。</span></div>
        <el-radio-group v-model="templateType"><el-radio-button label="metric">PromQL 规则（10）</el-radio-button><el-radio-button label="log">ES 日志规则（10）</el-radio-button></el-radio-group>
      </div>
      <div class="rule-template-grid">
        <button v-for="item in visibleRuleTemplates" :key="item.id" type="button" class="rule-template-card" @click="applyRuleTemplate(item)">
          <div><el-tag :type="item.type === 'log' ? 'warning' : 'primary'" size="small">{{ item.type === 'log' ? 'ES 日志' : 'PromQL' }}</el-tag><el-tag effect="plain" size="small">{{ item.severity }}</el-tag></div>
          <strong>{{ item.name }}</strong>
          <p>{{ item.description }}</p>
          <code>{{ item.queryText }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">取消</el-button></template>
    </el-dialog>

    <el-dialog v-model="batchNotifyVisible" title="批量启用通知" width="520px" append-to-body>
      <p class="batch-dialog-tip">将为已选中的 {{ selectedRuleIds.length }} 条告警规则开启消息通知，并统一绑定以下通知规则。</p>
      <el-form label-width="88px">
        <el-form-item label="通知规则" required>
          <el-select v-model="batchNotifyRuleId" filterable style="width: 100%" placeholder="请选择通知规则">
            <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="batchNotifyVisible = false">取消</el-button><el-button type="primary" :loading="batchNotifySaving" @click="submitBatchNotify">确认启用</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.header-actions { display: flex; gap: 10px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.batch-dialog-tip { margin: 0 0 18px; color: #7282a0; line-height: 1.65; }
.pager { display: flex; justify-content: flex-end; }
.datasource-scope { display: flex; align-items: center; gap: 14px; width: 100%; }
.form-tip { margin-top: 6px; color: #8491a9; font-size: 12px; line-height: 1.6; }
.datasource-scope .form-tip { margin: 0; }
.unit { margin-left: 6px; color: #8491a9; white-space: nowrap; }
.inline-tip { margin: 0 0 0 10px; }
.rule-parameters { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; margin: 2px 0 20px 120px; }
.parameter-card { min-height: 112px; padding: 14px; border: 1px solid #dfe7f2; border-radius: 8px; background: #f8faff; display: flex; flex-direction: column; gap: 8px; }
.parameter-card > span { color: #51617b; font-size: 13px; font-weight: 600; }
.parameter-card :deep(.el-input-number), .parameter-card :deep(.el-select) { width: 100%; }
.parameter-card small { color: #8c98ad; font-size: 12px; line-height: 1.4; }
.parameter-number { display: flex; align-items: center; gap: 6px; }
.parameter-number :deep(.el-input-number) { flex: 1; min-width: 0; }
.parameter-number b { color: #71809b; font-size: 12px; font-weight: 500; }
.template-dialog-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 16px; }
.template-dialog-head strong { display: block; color: #1e3155; }
.template-dialog-head span { display: block; margin-top: 6px; color: #8491a9; font-size: 13px; }
.rule-template-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; max-height: 530px; overflow: auto; padding-right: 4px; }
.rule-template-card { min-height: 160px; padding: 16px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .2s, box-shadow .2s; }
.rule-template-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.rule-template-card :deep(.el-tag) + :deep(.el-tag) { margin-left: 6px; }
.rule-template-card strong { display: block; margin-top: 12px; color: #1d3154; }
.rule-template-card p { min-height: 36px; margin: 7px 0; color: #7282a0; font-size: 13px; line-height: 1.45; }
.rule-template-card code { display: block; overflow: hidden; color: #4567c7; font-family: Consolas, Monaco, monospace; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 900px) { .rule-parameters { grid-template-columns: repeat(2, minmax(0, 1fr)); margin-left: 0; } .template-dialog-head { align-items: flex-start; flex-direction: column; } }
</style>
