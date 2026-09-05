<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { mt } from '../../utils/monitor-i18n'
import {
  batchUpdateMonitorAggregationRules,
  deleteMonitorAggregationRule,
  monitorAggregationRuleInfo,
  queryMonitorAggregationRuleList,
  queryMonitorAlertRuleList,
  saveMonitorAggregationRule
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const templateDialogVisible = ref(false)
const templateType = ref('metric')
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const ruleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', status: '' })

const aggregationTemplates = [
  { id: 'metric-instance', type: 'metric', name: mt('atlMetricInstanceName'), matchMode: 'regex', ruleNamePattern: '^Host.*', severity: '', groupBy: ['instance'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: mt('atlMetricInstanceDesc') },
  { id: 'metric-k8s-pod', type: 'metric', name: mt('atlMetricK8sPodName'), matchMode: 'regex', ruleNamePattern: '^(Kubernetes|Pod|Deployment|PVC).*', severity: '', groupBy: ['namespace', 'pod'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: mt('atlMetricK8sPodDesc') },
  { id: 'metric-target', type: 'metric', name: mt('atlMetricTargetName'), matchMode: 'regex', ruleNamePattern: '^수집 대상.*', severity: 'P1', groupBy: ['instance', 'job'], windowSeconds: 600, repeatIntervalSeconds: 3600, description: mt('atlMetricTargetDesc') },
  { id: 'metric-service', type: 'metric', name: mt('atlMetricServiceName'), matchMode: 'select', ruleIds: [], severity: '', groupBy: ['service', 'namespace'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: mt('atlMetricServiceDesc') },
  { id: 'metric-low-priority', type: 'metric', name: mt('atlMetricLowPriorityName'), matchMode: 'select', ruleIds: [], severity: 'P3', groupBy: ['instance', 'alertname'], windowSeconds: 900, repeatIntervalSeconds: 7200, description: mt('atlMetricLowPriorityDesc') },
  { id: 'log-service', type: 'log', name: mt('atlLogServiceName'), matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', groupBy: ['service', 'namespace'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: mt('atlLogServiceDesc') },
  { id: 'log-pod', type: 'log', name: mt('atlLogPodName'), matchMode: 'select', ruleIds: [], severity: '', groupBy: ['namespace', 'pod'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: mt('atlLogPodDesc') },
  { id: 'log-datasource', type: 'log', name: mt('atlLogDatasourceName'), matchMode: 'select', ruleIds: [], severity: '', groupBy: ['datasource', 'index'], windowSeconds: 600, repeatIntervalSeconds: 3600, description: mt('atlLogDatasourceDesc') },
  { id: 'log-critical', type: 'log', name: mt('atlLogCriticalName'), matchMode: 'select', ruleIds: [], severity: 'P1', groupBy: ['service', 'alertname'], windowSeconds: 120, repeatIntervalSeconds: 900, description: mt('atlLogCriticalDesc') },
  { id: 'log-low-priority', type: 'log', name: mt('atlLogLowPriorityName'), matchMode: 'select', ruleIds: [], severity: 'P3', groupBy: ['service', 'alertname'], windowSeconds: 900, repeatIntervalSeconds: 7200, description: mt('atlLogLowPriorityDesc') }
]

const visibleAggregationTemplates = computed(() => aggregationTemplates.filter((item) => item.type === templateType.value))

const form = reactive({
  id: undefined,
  name: '',
  matchMode: 'select',
  ruleIds: [],
  ruleNamePattern: '',
  severity: '',
  alertType: '',
  groupBy: [],
  windowSeconds: 300,
  repeatIntervalSeconds: 1800,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    matchMode: 'select',
    ruleIds: [],
    ruleNamePattern: '',
    severity: '',
    alertType: '',
    groupBy: [],
    windowSeconds: 300,
    repeatIntervalSeconds: 1800,
    status: 1,
    description: ''
  })
}

function normalizePayload() {
  return {
    ...form,
    ruleIds: form.matchMode === 'select' ? form.ruleIds : [],
    ruleNamePattern: form.matchMode === 'regex' ? form.ruleNamePattern : ''
  }
}

function selectedRuleNames(ids = []) {
  if (!ids.length) return mt('allRules')
  return ids
    .map((id) => ruleOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join(', ')
}

async function loadRuleOptions() {
  const data = await queryMonitorAlertRuleList({ pageNum: 1, pageSize: 1000 })
  ruleOptions.value = data.list || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAggregationRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateType.value = 'metric'
  templateDialogVisible.value = true
}

function applyAggregationTemplate(template) {
  resetForm()
  Object.assign(form, {
    name: template.name,
    matchMode: template.matchMode,
    ruleIds: template.ruleIds || [],
    ruleNamePattern: template.ruleNamePattern || '',
    severity: template.severity || '',
    alertType: template.type === 'log' ? 'log' : 'metric',
    groupBy: template.groupBy,
    windowSeconds: template.windowSeconds,
    repeatIntervalSeconds: template.repeatIntervalSeconds,
    description: template.description
  })
  templateDialogVisible.value = false
  dialogVisible.value = true
}

function handleSelectionChange(selection) {
  selectedRuleIds.value = selection.map((item) => item.id)
}

async function handleBatchAction(action) {
  if (!selectedRuleIds.value.length) return
  const labels = { enable: mt('enabledOption'), disable: mt('disabledOption'), delete: mt('delete') }
  await ElMessageBox.confirm(mt('aggBatchConfirm', { count: selectedRuleIds.value.length, action: labels[action] }), mt('batchStatusTitle'), { type: action === 'delete' ? 'warning' : 'info' })
  await batchUpdateMonitorAggregationRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(mt('silenceBatchDone', { action: labels[action] }))
  selectedRuleIds.value = []
  await loadData()
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorAggregationRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    matchMode: data.matchMode || 'regex',
    ruleIds: data.ruleIds || [],
    groupBy: data.groupBy || []
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning(mt('enterAggName'))
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning(mt('enterRulePattern'))
    return
  }
  saving.value = true
  try {
    await saveMonitorAggregationRule(normalizePayload())
    ElMessage.success(mt('savedMsg'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(mt('aggDeleteConfirm', { name: row.name }), mt('noticeTitle'), { type: 'warning' })
  await deleteMonitorAggregationRule(row.id)
  ElMessage.success(mt('deletedMsg'))
  await loadData()
}

onMounted(async () => {
  await loadRuleOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page monitor-aggregation-page">
    <div class="page-header">
      <div>
        <h2>{{ mt('aggRuleTitle') }}</h2>
        <p>{{ mt('aggPageDesc') }}</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">{{ mt('importCommonTemplates') }}</el-button><el-button type="primary" @click="openCreate">{{ mt('newAggRule') }}</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="mt('silenceSearchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable :placeholder="mt('status')" style="width: 120px">
        <el-option :label="mt('enabledOption')" value="1" />
        <el-option :label="mt('disabledOption')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ mt('searchLabel') }}</el-button>
    </div>

    <div v-if="selectedRuleIds.length" class="batch-toolbar">
      <span>{{ mt('selectedAggCount', { count: selectedRuleIds.length }) }}</span>
      <el-button size="small" type="success" @click="handleBatchAction('enable')">{{ mt('batchEnable') }}</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('disable')">{{ mt('batchDisable') }}</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">{{ mt('batchDelete') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" :label="mt('nameLabel')" min-width="180" />
      <el-table-column :label="mt('ruleMatching')" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">{{ mt('multiSelectColon', { names: selectedRuleNames(row.ruleIds) }) }}</span>
          <span v-else>{{ mt('regexColon', { pattern: row.ruleNamePattern || '-' }) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="Severity" width="90">
        <template #default="{ row }">{{ row.severity || mt('allShort') }}</template>
      </el-table-column>
      <el-table-column prop="alertType" :label="mt('alertTypeCol2')" width="130"><template #default="{ row }">{{ ({ metric: mt('metricShort'), log: mt('esLogShort'), victorialogs: 'VictoriaLogs' }[row.alertType] || mt('allShort')) }}</template></el-table-column>
      <el-table-column :label="mt('groupFieldsCol')" min-width="220">
        <template #default="{ row }">
          <template v-if="row.groupBy?.length"><el-tag v-for="item in row.groupBy" :key="item" class="tag">{{ item }}</el-tag></template><el-tag v-else type="warning" effect="light">{{ mt('noGroupBy') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="mt('convergeWindowCol')" width="130">
        <template #default="{ row }">{{ mt('secondsValue', { count: row.windowSeconds }) }}</template>
      </el-table-column>
      <el-table-column :label="mt('dupNotifyIntervalCol')" width="150">
        <template #default="{ row }">{{ mt('secondsValue', { count: row.repeatIntervalSeconds }) }}</template>
      </el-table-column>
      <el-table-column :label="mt('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? mt('enabledOption') : mt('disabledOption') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="mt('actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ mt('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ mt('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? mt('editAggRule') : mt('newAggRuleFull')" width="820px">
      <el-form label-width="140px">
        <el-form-item :label="mt('nameLabel')" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="mt('ruleNameMatching')">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">{{ mt('dropdownMultiRule') }}</el-radio-button>
            <el-radio-button label="regex">{{ mt('regexRuleNameMatch') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="Alert Rule">
          <el-select v-model="form.ruleIds" multiple filterable clearable :placeholder="mt('ruleIdsPlaceholderAgg')" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else :label="mt('ruleNamePatternLabel')" required>
          <el-input v-model="form.ruleNamePattern" :placeholder="mt('regexPlaceholderAgg')" />
        </el-form-item>
        <el-form-item label="Severity">
          <el-select v-model="form.severity" clearable :placeholder="mt('allSeverity')" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item :label="mt('alertTypeCol2')"><el-select v-model="form.alertType" clearable :placeholder="mt('allTypes')" style="width: 100%"><el-option :label="mt('metricAlertType')" value="metric" /><el-option :label="mt('esLogAlertType')" value="log" /><el-option :label="mt('vlAlertType')" value="victorialogs" /></el-select></el-form-item>
        <el-form-item :label="mt('groupFieldsOptional')">
          <el-select v-model="form.groupBy" multiple filterable allow-create default-first-option clearable style="width: 100%" :placeholder="mt('groupByPlaceholder')">
            <el-option v-for="item in ['instance','job','namespace','pod','service','cluster']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-alert type="info" :closable="false" :title="mt('groupByAlert')" />
        <el-form-item :label="mt('convergeWindowCol')">
          <div class="number-with-unit">
            <el-input-number v-model="form.windowSeconds" :min="60" :max="86400" />
            <span>{{ mt('secondUnit') }}</span>
          </div>
        </el-form-item>
        <el-form-item :label="mt('dupNotifyIntervalCol')">
          <div class="number-with-unit">
            <el-input-number v-model="form.repeatIntervalSeconds" :min="60" :max="86400" />
            <span>{{ mt('secondUnit') }}</span>
          </div>
        </el-form-item>
        <el-form-item :label="mt('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ mt('enabledOption') }}</el-radio>
            <el-radio :value="2">{{ mt('disabledOption') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ mt('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ mt('save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" :title="mt('importAggTemplatesTitle')" width="780px">
      <div class="template-head"><span>{{ mt('aggTemplateHint') }}</span><el-radio-group v-model="templateType"><el-radio-button label="metric">{{ mt('metricAlertTemplate') }}</el-radio-button><el-radio-button label="log">{{ mt('logAlertTemplate') }}</el-radio-button></el-radio-group></div>
      <div class="template-grid">
        <button v-for="item in visibleAggregationTemplates" :key="item.id" type="button" class="template-card" @click="applyAggregationTemplate(item)">
          <el-tag :type="item.type === 'log' ? 'warning' : 'primary'" size="small">{{ item.type === 'log' ? mt('logAlertType') : mt('metricAlertType') }}</el-tag>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ mt('groupBySummary', { fields: item.groupBy.join(', '), seconds: item.windowSeconds }) }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">{{ mt('cancel') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px;
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08);
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.header-actions { display: flex; gap: 10px; }
.page-header h2 {
  margin: 0 0 8px;
  font-size: 26px;
  color: #10213f;
}
.page-header p {
  margin: 0;
  color: #7282a0;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.pager {
  display: flex;
  justify-content: flex-end;
}
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.template-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; color: #7282a0; font-size: 13px; }
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.template-card { min-height: 138px; padding: 14px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.template-card strong { display: block; margin-top: 10px; color: #1d3154; }
.template-card p { min-height: 35px; margin: 6px 0; color: #7282a0; font-size: 13px; line-height: 1.4; }
.template-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.tag {
  margin-right: 6px;
}
.number-with-unit {
  display: flex;
  align-items: center;
  gap: 10px;
}
.number-with-unit .el-input-number {
  width: 220px;
}
.number-with-unit span {
  color: #64748b;
}
</style>
