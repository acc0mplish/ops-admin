<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  batchUpdateMonitorSilenceRules,
  deleteMonitorSilenceRule,
  monitorSilenceRuleInfo,
  previewMonitorSilenceRule,
  queryMonitorAlertRuleList,
  queryMonitorSilenceRuleList,
  saveMonitorSilenceRule
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const templateDialogVisible = ref(false)
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewData = ref({ matchedRuleCount: 0, matchedRules: [], matchedActiveEventCount: 0, matchedActiveEvents: [] })
const templateType = ref('metric')
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const ruleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', status: '' })
const timeRange = ref([])

const silenceTemplates = [
  { id: 'metric-maintenance', type: 'metric', name: '모니터링 알림 - Maintenance Window', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{}', description: 'Maintenance Window 동안 전체 모니터링 알림을 차단합니다. 차단 시간을 설정하십시오.' },
  { id: 'metric-node', type: 'metric', name: '모니터링 알림 - Host 리소스 유지보수', matchMode: 'regex', ruleNamePattern: '^Host.*(CPU|메모리|디스크|부하).*', severity: '', matchersJson: '{}', description: 'Host 리소스 유지보수 관련 모니터링 알림을 차단합니다.' },
  { id: 'metric-k8s', type: 'metric', name: '모니터링 알림 - Kubernetes Deploy Window', matchMode: 'regex', ruleNamePattern: '^(Kubernetes|Pod|Deployment|PVC).*', severity: '', matchersJson: '{}', description: 'Deploy Window 동안 Kubernetes Workload 알림을 차단합니다.' },
  { id: 'metric-instance', type: 'metric', name: '모니터링 알림 - 지정 Host 유지보수', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"instance":"10.0.0.1"}', description: 'instance를 교체한 뒤 지정 Host의 모니터링 알림을 차단합니다.' },
  { id: 'metric-low-priority', type: 'metric', name: '모니터링 알림 - 낮은 우선순위 차단', matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{}', description: 'P3 낮은 우선순위 모니터링 알림을 임시 차단합니다.' },
  { id: 'es-maintenance', type: 'elasticsearch', name: 'Elasticsearch 로그 알림 - Maintenance Window', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"log"}', description: 'Maintenance Window 동안 전체 Elasticsearch 로그 알림을 차단합니다.' },
  { id: 'es-error', type: 'elasticsearch', name: 'Elasticsearch 로그 알림 - ERROR Deploy 노이즈', matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', matchersJson: '{"alert_type":"log"}', description: 'Deploy 기간 동안 ERROR와 오류 유형의 Elasticsearch 로그 알림을 차단합니다.' },
  { id: 'es-datasource', type: 'elasticsearch', name: 'Elasticsearch 로그 알림 - 지정 Datasource', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"log","datasource":"로그 Datasource 이름"}', description: 'datasource를 교체한 뒤 지정 Elasticsearch 로그 Datasource를 차단합니다.' },
  { id: 'es-low-priority', type: 'elasticsearch', name: 'Elasticsearch 로그 알림 - 낮은 우선순위 차단', matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{"alert_type":"log"}', description: 'P3 낮은 우선순위 Elasticsearch 로그 알림을 임시 차단합니다.' },
  { id: 'vl-maintenance', type: 'victorialogs', name: 'VictoriaLogs 로그 알림 - Maintenance Window', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"victorialogs"}', description: 'Maintenance Window 동안 전체 VictoriaLogs 로그 알림을 차단합니다.' },
  { id: 'vl-error', type: 'victorialogs', name: 'VictoriaLogs 로그 알림 - ERROR Deploy 노이즈', matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', matchersJson: '{"alert_type":"victorialogs"}', description: 'Deploy 기간 동안 ERROR와 오류 유형의 VictoriaLogs 로그 알림을 차단합니다.' },
  { id: 'vl-datasource', type: 'victorialogs', name: 'VictoriaLogs 로그 알림 - 지정 Datasource', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"victorialogs","datasource":"로그 Datasource 이름"}', description: 'datasource를 교체한 뒤 지정 VictoriaLogs 로그 Datasource를 차단합니다.' },
  { id: 'vl-low-priority', type: 'victorialogs', name: 'VictoriaLogs 로그 알림 - 낮은 우선순위 차단', matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{"alert_type":"victorialogs"}', description: 'P3 낮은 우선순위 VictoriaLogs 로그 알림을 임시 차단합니다.' }
]

const visibleSilenceTemplates = computed(() => silenceTemplates.filter((item) => item.type === templateType.value))

const form = reactive({
  id: undefined,
  name: '',
  matchMode: 'select',
  ruleIds: [],
  ruleNamePattern: '',
  severity: '',
  alertType: '',
  matchersJson: '{}',
  startsAt: 0,
  endsAt: 0,
  priority: 100,
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
    matchersJson: '{}',
    startsAt: 0,
    endsAt: 0,
    priority: 100,
    status: 1,
    description: ''
  })
  timeRange.value = []
}

function toUnixSeconds(value) {
  return value ? Math.floor(new Date(value).getTime() / 1000) : 0
}

function normalizePayload() {
  const [start, end] = timeRange.value || []
  return {
    ...form,
    ruleIds: form.matchMode === 'select' ? form.ruleIds : [],
    ruleNamePattern: form.matchMode === 'regex' ? form.ruleNamePattern : '',
    startsAt: toUnixSeconds(start),
    endsAt: toUnixSeconds(end)
  }
}

function silenceSchedule(row) {
  if (row.status !== 1) return { text: '비활성화됨', type: 'info' }
  const now = Date.now()
  const start = row.startsAt ? new Date(row.startsAt).getTime() : 0
  const end = row.endsAt ? new Date(row.endsAt).getTime() : 0
  if (start && start > now) return { text: '적용 대기', type: 'warning' }
  if (end && end < now) return { text: '만료', type: 'info' }
  return { text: '적용 중', type: 'success' }
}

function selectedRuleNames(ids = []) {
  if (!ids.length) return '전체 Rule'
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
    const data = await queryMonitorSilenceRuleList(query)
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

function applySilenceTemplate(template) {
  resetForm()
  Object.assign(form, {
    name: template.name,
    matchMode: template.matchMode,
    ruleIds: template.ruleIds || [],
    ruleNamePattern: template.ruleNamePattern || '',
    severity: template.severity || '',
    alertType: template.type === 'elasticsearch' ? 'log' : template.type,
    matchersJson: template.matchersJson,
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
  const labels = { enable: '활성화', disable: '비활성화', delete: '삭제' }
  await ElMessageBox.confirm(`선택한 ${selectedRuleIds.value.length}건 차단 Rule을 일괄 ${labels[action]}하시겠습니까?`, '일괄 작업 확인', { type: action === 'delete' ? 'warning' : 'info' })
  await batchUpdateMonitorSilenceRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(`일괄 ${labels[action]}했습니다.`)
  selectedRuleIds.value = []
  await loadData()
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorSilenceRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    matchMode: data.matchMode || 'regex',
    ruleIds: data.ruleIds || [],
    matchersJson: data.matchersJson || '{}'
  })
  timeRange.value = data.startsAt && data.endsAt ? [data.startsAt, data.endsAt] : []
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('차단 Rule 이름을 입력하십시오.')
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning('Rule 이름 정규식을 입력하십시오.')
    return
  }
  const [start, end] = timeRange.value || []
  if (start && end && new Date(end).getTime() <= new Date(start).getTime()) {
    ElMessage.warning('종료 시각은 시작 시각보다 늦어야 합니다.')
    return
  }
  const isGlobalPermanentSilence = form.matchMode === 'select' && !form.ruleIds.length && !form.alertType && !form.severity && form.matchersJson.trim() === '{}' && !start && !end
  if (isGlobalPermanentSilence) {
    await ElMessageBox.confirm('이 Rule은 모든 유형과 모든 Severity의 알림을 즉시 차단하며 자동으로 종료되지 않습니다. 알림 유형, 매칭 범위 또는 종료 시각을 제한하는 것을 권장합니다. 그래도 저장하시겠습니까?', '고위험 전역 차단', { type: 'warning', confirmButtonText: '그래도 저장', cancelButtonText: '수정으로 돌아가기' })
  }
  saving.value = true
  try {
    await saveMonitorSilenceRule(normalizePayload())
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function openPreview() {
  if (!form.name.trim()) {
    ElMessage.warning('먼저 차단 Rule 이름을 입력하십시오.')
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning('Rule 이름 정규식을 입력하십시오.')
    return
  }
  previewLoading.value = true
  try {
    previewData.value = await previewMonitorSilenceRule(normalizePayload())
    previewVisible.value = true
  } finally {
    previewLoading.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`차단 Rule "${row.name}"을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteMonitorSilenceRule(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

onMounted(async () => {
  await loadRuleOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page monitor-silence-page">
    <div class="page-header">
      <div>
        <h2>Alert 차단 Rule</h2>
        <p>특정 Alert Rule 또는 Rule 이름 정규식으로 알림을 차단합니다. 매칭 시 Event만 기록하며 Notification은 전송하지 않습니다.</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">자주 쓰는 Template Import</el-button><el-button type="primary" @click="openCreate">새 차단 Rule</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="이름 / Rule 이름 검색" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 120px">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <div v-if="selectedRuleIds.length" class="batch-toolbar">
      <span>차단 Rule <b>{{ selectedRuleIds.length }}</b>건을 선택했습니다</span>
      <el-button size="small" type="success" @click="handleBatchAction('enable')">일괄 활성화</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('disable')">일괄 비활성화</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">일괄 삭제</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" label="이름" min-width="180" />
      <el-table-column label="Rule 매칭" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">다중 선택: {{ selectedRuleNames(row.ruleIds) }}</span>
          <span v-else>정규식: {{ row.ruleNamePattern || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="Severity" width="90">
        <template #default="{ row }">{{ row.severity || '전체' }}</template>
      </el-table-column>
      <el-table-column prop="alertType" label="알림 유형" width="130"><template #default="{ row }">{{ ({ metric: '모니터링', log: 'ES 로그', victorialogs: 'VictoriaLogs' }[row.alertType] || '전체') }}</template></el-table-column>
      <el-table-column prop="matchersJson" label="Label 매칭" min-width="220" show-overflow-tooltip />
      <el-table-column prop="startsAt" label="시작 시각" width="180" />
      <el-table-column prop="endsAt" label="종료 시각" width="180" />
      <el-table-column label="적용 상태" width="100"><template #default="{ row }"><el-tag :type="silenceSchedule(row).type">{{ silenceSchedule(row).text }}</el-tag></template></el-table-column>
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="작업" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '차단 Rule 수정' : '새 차단 Rule'" width="820px">
      <el-form label-width="120px">
        <el-form-item label="이름" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Rule 매칭">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">드롭다운 다중 선택</el-radio-button>
            <el-radio-button label="regex">Rule 이름 정규식 매칭</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="Alert Rule">
          <el-select v-model="form.ruleIds" multiple filterable clearable placeholder="비워두면 전체 Alert Rule 대상" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="Rule 이름 정규식" required>
          <el-input v-model="form.ruleNamePattern" placeholder="예: ^skzy-sh.*은(는) skzy-sh로 시작하는 모든 Alert Rule과 매칭합니다" />
        </el-form-item>
        <el-form-item label="Severity">
          <el-select v-model="form.severity" clearable placeholder="전체 Severity" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="알림 유형"><el-select v-model="form.alertType" clearable placeholder="전체 유형" style="width: 100%"><el-option label="모니터링 알림" value="metric" /><el-option label="ES 로그 알림" value="log" /><el-option label="VictoriaLogs 알림" value="victorialogs" /></el-select></el-form-item>
        <el-form-item label="Label 매칭">
          <el-input v-model="form.matchersJson" type="textarea" :rows="3" placeholder='예: {"instance":"10.0.0.1","job":"node"}' />
        </el-form-item>
        <el-form-item label="차단 시간">
          <el-date-picker v-model="timeRange" type="datetimerange" start-placeholder="시작 시각" end-placeholder="종료 시각" value-format="YYYY-MM-DD HH:mm:ss" style="width: 100%" />
        </el-form-item>
        <el-form-item label="우선순위"><el-input-number v-model="form.priority" :min="1" :max="1000" /><span class="form-tip">값이 클수록 우선합니다. 여러 Rule이 매칭되면 우선순위가 가장 높은 하나만 사용합니다.</span></el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">활성화</el-radio>
            <el-radio :value="2">비활성화</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="설명"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button :loading="previewLoading" @click="openPreview">매칭 미리보기</el-button>
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewVisible" title="차단 매칭 미리보기" width="760px">
      <div class="preview-summary"><el-tag type="primary">Alert Rule {{ previewData.matchedRuleCount || 0 }}건 매칭</el-tag><el-tag type="warning">현재 활성 Event {{ previewData.matchedActiveEventCount || 0 }}건 영향</el-tag></div>
      <el-alert type="info" :closable="false" title="미리보기는 현재 Rule과 활성 Event 기준입니다. 실제 적용은 차단 시간, Severity, Label 조건이 함께 제약합니다." />
      <h4>매칭된 Alert Rule</h4>
      <el-table :data="previewData.matchedRules || []" max-height="220" size="small"><el-table-column prop="name" label="Rule 이름" /><el-table-column prop="alertType" label="유형" width="130" /><el-table-column prop="severity" label="Severity" width="90" /></el-table>
      <h4>현재 영향 받는 Event</h4>
      <el-table :data="previewData.matchedActiveEvents || []" max-height="220" size="small"><el-table-column prop="ruleName" label="Rule" /><el-table-column prop="status" label="상태" width="100" /><el-table-column prop="summary" label="요약" show-overflow-tooltip /></el-table>
      <template #footer><el-button type="primary" @click="previewVisible = false">확인</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" title="자주 쓰는 차단 Template Import" width="780px">
      <div class="template-head"><span>Template은 Rule 매칭, Severity, Label 매칭 조건을 가져오므로 실제 환경에 맞게 수정하십시오.</span><el-radio-group v-model="templateType"><el-radio-button label="metric">모니터링 알림</el-radio-button><el-radio-button label="elasticsearch">Elasticsearch</el-radio-button><el-radio-button label="victorialogs">VictoriaLogs</el-radio-button></el-radio-group></div>
      <div class="template-grid">
        <button v-for="item in visibleSilenceTemplates" :key="item.id" type="button" class="template-card" @click="applySilenceTemplate(item)">
           <el-tag :type="item.type === 'metric' ? 'primary' : item.type === 'elasticsearch' ? 'warning' : 'success'" size="small">{{ item.type === 'metric' ? '모니터링 알림' : item.type === 'elasticsearch' ? 'Elasticsearch' : 'VictoriaLogs' }}</el-tag>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.matchMode === 'regex' ? item.ruleNamePattern : item.matchersJson }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">취소</el-button></template>
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
.pager { display: flex; justify-content: flex-end; }
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.template-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; color: #7282a0; font-size: 13px; }
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.template-card { min-height: 138px; padding: 14px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.template-card strong { display: block; margin-top: 10px; color: #1d3154; }
.template-card p { min-height: 35px; margin: 6px 0; color: #7282a0; font-size: 13px; line-height: 1.4; }
.template-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.form-tip { margin-left: 10px; color: #8090ac; font-size: 12px; }
.preview-summary { display: flex; gap: 10px; margin-bottom: 14px; }
.preview-summary + .el-alert { margin-bottom: 14px; }
.el-dialog h4 { margin: 16px 0 8px; color: #20345b; }
</style>
