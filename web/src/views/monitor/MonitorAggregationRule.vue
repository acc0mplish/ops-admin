<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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
  { id: 'metric-instance', type: 'metric', name: '모니터링 알림 - Host 인스턴스 수렴', matchMode: 'regex', ruleNamePattern: '^Host.*', severity: '', groupBy: ['instance'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: 'CPU, 메모리, 디스크, 부하 유형의 알림을 Host 인스턴스별로 수렴합니다.' },
  { id: 'metric-k8s-pod', type: 'metric', name: '모니터링 알림 - Kubernetes Pod 수렴', matchMode: 'regex', ruleNamePattern: '^(Kubernetes|Pod|Deployment|PVC).*', severity: '', groupBy: ['namespace', 'pod'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: 'Namespace와 Pod별로 Kubernetes 리소스 알림을 수렴합니다.' },
  { id: 'metric-target', type: 'metric', name: '모니터링 알림 - 수집 대상 수렴', matchMode: 'regex', ruleNamePattern: '^수집 대상.*', severity: 'P1', groupBy: ['instance', 'job'], windowSeconds: 600, repeatIntervalSeconds: 3600, description: 'Instance와 수집 Job별로 Target 접속 불가 알림을 수렴합니다.' },
  { id: 'metric-service', type: 'metric', name: '모니터링 알림 - Service 차원 수렴', matchMode: 'select', ruleIds: [], severity: '', groupBy: ['service', 'namespace'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: 'Service와 Namespace별로 Application 모니터링 알림을 수렴합니다.' },
  { id: 'metric-low-priority', type: 'metric', name: '모니터링 알림 - P3 노이즈 감소 수렴', matchMode: 'select', ruleIds: [], severity: 'P3', groupBy: ['instance', 'alertname'], windowSeconds: 900, repeatIntervalSeconds: 7200, description: '낮은 우선순위 모니터링 알림을 한곳에 수렴해 반복 알림을 줄입니다.' },
  { id: 'log-service', type: 'log', name: '로그 알림 - Service 오류 수렴', matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', groupBy: ['service', 'namespace'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: 'Service와 Namespace별로 오류 로그 알림을 수렴합니다.' },
  { id: 'log-pod', type: 'log', name: '로그 알림 - Pod 차원 수렴', matchMode: 'select', ruleIds: [], severity: '', groupBy: ['namespace', 'pod'], windowSeconds: 300, repeatIntervalSeconds: 1800, description: '동일 Container의 중복 로그 알림을 Pod별로 수렴합니다.' },
  { id: 'log-datasource', type: 'log', name: '로그 알림 - Datasource 수렴', matchMode: 'select', ruleIds: [], severity: '', groupBy: ['datasource', 'index'], windowSeconds: 600, repeatIntervalSeconds: 3600, description: 'ES Datasource와 Index별로 동일 유형 로그 알림을 수렴합니다.' },
  { id: 'log-critical', type: 'log', name: '로그 알림 - P0/P1 빠른 수렴', matchMode: 'select', ruleIds: [], severity: 'P1', groupBy: ['service', 'alertname'], windowSeconds: 120, repeatIntervalSeconds: 900, description: '중요 로그 알림은 짧은 Window로 수렴하면서 빠른 반복 알림은 유지합니다.' },
  { id: 'log-low-priority', type: 'log', name: '로그 알림 - 낮은 우선순위 노이즈 감소', matchMode: 'select', ruleIds: [], severity: 'P3', groupBy: ['service', 'alertname'], windowSeconds: 900, repeatIntervalSeconds: 7200, description: '낮은 우선순위 로그 알림을 비교적 긴 시간 동안 수렴합니다.' }
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
  const labels = { enable: '활성화', disable: '비활성화', delete: '삭제' }
  await ElMessageBox.confirm(`선택한 ${selectedRuleIds.value.length}건의 집계 수렴 Rule을 일괄 ${labels[action]}하시겠습니까?`, '일괄 작업 확인', { type: action === 'delete' ? 'warning' : 'info' })
  await batchUpdateMonitorAggregationRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(`일괄 ${labels[action]}했습니다.`)
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
    ElMessage.warning('집계 Rule 이름을 입력하십시오.')
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning('Rule 이름 정규식을 입력하십시오.')
    return
  }
  saving.value = true
  try {
    await saveMonitorAggregationRule(normalizePayload())
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`집계 수렴 Rule "${row.name}"을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteMonitorAggregationRule(row.id)
  ElMessage.success('삭제했습니다.')
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
        <h2>Alert 집계 수렴 Rule</h2>
        <p>Alert Rule, Severity, Label 필드별로 동일 유형 알림을 집계해 수렴 Window 내 중복 Notification을 줄입니다.</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">자주 쓰는 Template Import</el-button><el-button type="primary" @click="openCreate">새 집계 Rule</el-button></div>
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
      <span>집계 수렴 Rule <b>{{ selectedRuleIds.length }}</b>건을 선택했습니다</span>
      <el-button size="small" type="success" @click="handleBatchAction('enable')">일괄 활성화</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('disable')">일괄 비활성화</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">일괄 삭제</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" label="이름" min-width="180" />
      <el-table-column label="Rule 매칭" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">다중 선택: {{ selectedRuleNames(row.ruleIds) }}</span>
          <span v-else>정규식: {{ row.ruleNamePattern || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="Severity" width="90">
        <template #default="{ row }">{{ row.severity || '전체' }}</template>
      </el-table-column>
      <el-table-column prop="alertType" label="Alert 유형" width="130"><template #default="{ row }">{{ ({ metric: '모니터링', log: 'ES 로그', victorialogs: 'VictoriaLogs' }[row.alertType] || '전체') }}</template></el-table-column>
      <el-table-column label="그룹 필드" min-width="220">
        <template #default="{ row }">
          <template v-if="row.groupBy?.length"><el-tag v-for="item in row.groupBy" :key="item" class="tag">{{ item }}</el-tag></template><el-tag v-else type="warning" effect="light">Label로 그룹화하지 않음</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="수렴 Window" width="130">
        <template #default="{ row }">{{ row.windowSeconds }}초</template>
      </el-table-column>
      <el-table-column label="중복 Notification 간격" width="150">
        <template #default="{ row }">{{ row.repeatIntervalSeconds }}초</template>
      </el-table-column>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '집계 수렴 Rule 수정' : '새 집계 수렴 Rule'" width="820px">
      <el-form label-width="140px">
        <el-form-item label="이름" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Rule 이름 매칭">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">드롭다운 다중 선택 Rule</el-radio-button>
            <el-radio-button label="regex">정규식으로 Rule 이름 매칭</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="Alert Rule">
          <el-select v-model="form.ruleIds" multiple filterable clearable placeholder="비워두면 전체 Alert Rule에 적용" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="Rule 이름 정규식" required>
          <el-input v-model="form.ruleNamePattern" placeholder="예: ^skzy-sh.* 는 skzy-sh로 시작하는 모든 Alert Rule에 매칭됩니다" />
        </el-form-item>
        <el-form-item label="Severity">
          <el-select v-model="form.severity" clearable placeholder="전체 Severity" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="Alert 유형"><el-select v-model="form.alertType" clearable placeholder="전체 유형" style="width: 100%"><el-option label="모니터링 알림" value="metric" /><el-option label="ES 로그 알림" value="log" /><el-option label="VictoriaLogs 알림" value="victorialogs" /></el-select></el-form-item>
        <el-form-item label="그룹 필드 (선택)">
          <el-select v-model="form.groupBy" multiple filterable allow-create default-first-option clearable style="width: 100%" placeholder="비워두면 동일 Rule, 동일 Severity의 알림을 한 건의 Notification으로 요약">
            <el-option v-for="item in ['instance','job','namespace','pod','service','cluster']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-alert type="info" :closable="false" title="비워두기: 동일 Rule, 동일 Severity의 알림이 Window 내에서 한 건의 요약 Notification으로 합쳐집니다. 필드 선택: 필드 값이 동일한 알림만 병합됩니다. 안정적으로 존재하는 Label만 선택하십시오." />
        <el-form-item label="수렴 Window">
          <div class="number-with-unit">
            <el-input-number v-model="form.windowSeconds" :min="60" :max="86400" />
            <span>초</span>
          </div>
        </el-form-item>
        <el-form-item label="중복 Notification 간격">
          <div class="number-with-unit">
            <el-input-number v-model="form.repeatIntervalSeconds" :min="60" :max="86400" />
            <span>초</span>
          </div>
        </el-form-item>
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
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" title="자주 쓰는 집계 수렴 Template Import" width="780px">
      <div class="template-head"><span>Template은 Rule 매칭, 그룹 필드, 수렴 Window, 중복 Notification 간격을 가져옵니다.</span><el-radio-group v-model="templateType"><el-radio-button label="metric">모니터링 알림 Template</el-radio-button><el-radio-button label="log">로그 알림 Template</el-radio-button></el-radio-group></div>
      <div class="template-grid">
        <button v-for="item in visibleAggregationTemplates" :key="item.id" type="button" class="template-card" @click="applyAggregationTemplate(item)">
          <el-tag :type="item.type === 'log' ? 'warning' : 'primary'" size="small">{{ item.type === 'log' ? '로그 알림' : '모니터링 알림' }}</el-tag>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>그룹: {{ item.groupBy.join(', ') }} / {{ item.windowSeconds }}초</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">취소</el-button></template>
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
