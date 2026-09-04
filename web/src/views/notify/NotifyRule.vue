<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyRule,
  notifyRuleInfo,
  queryNotifyChannelOptions,
  queryNotifyRuleList,
  queryNotifyTemplateOptions,
  saveNotifyRule,
  testNotifyRule
} from '../../api/ops'

const loading = ref(false)
const saving = ref(false)
const optionsLoading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const activeStep = ref(0)
const rows = ref([])
const total = ref(0)
const templateOptions = ref([])
const channelOptions = ref([])

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', scope: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  scope: 'monitor',
  events: ['firing', 'recovered'],
  templateId: undefined,
  channelIds: [],
  status: 1,
  description: ''
})

const scopeOptions = [
  { label: 'Job Orchestration', value: 'job' },
  { label: 'CI/CD Pipeline', value: 'pipeline' },
  { label: 'Schedule Task', value: 'schedule' },
  { label: 'Monitor Alert', value: 'monitor' }
]
const filterScopeOptions = scopeOptions

const eventOptions = [
  { label: '성공', value: 'success' },
  { label: '실패', value: 'failed' },
  { label: '수동 확인 대기', value: 'waiting_approval' },
  { label: '거부됨', value: 'rejected' },
  { label: 'Alert 발생', value: 'firing' },
  { label: 'Alert 복구', value: 'recovered' },
  { label: '전체 Event', value: 'all' }
]

const channelTypeLabels = {
  dingtalk: 'DingTalk Bot',
  wecom: 'WeCom Bot',
  feishu: 'Feishu Bot',
  webhook: '사용자 정의 Webhook'
}

const selectedTemplate = computed(() => templateOptions.value.find((item) => Number(item.id) === Number(form.templateId)))
const compatibleTemplateOptions = computed(() => templateOptions.value.filter((item) => {
  const templateScope = item.scope || 'all'
  if (form.scope === 'all') return true
  if (templateScope === 'all') return true
  return templateScope === form.scope
}))
const selectedChannels = computed(() => channelOptions.value.filter((item) => form.channelIds.map(Number).includes(Number(item.id))))
const wizardSteps = [
  { title: '트리거 시나리오', description: '언제 알림을 보낼지 선택' },
  { title: '전송 방식', description: 'Template과 Channel 선택' },
  { title: '활성화 확인', description: 'Routing을 확인하고 저장' }
]
const routeWarnings = computed(() => {
  const warnings = []
  if (!form.scope || form.scope === 'all') warnings.push('구체적인 비즈니스 시나리오를 지정하십시오')
  if (!form.events.length) warnings.push('하나 이상의 트리거 Event를 선택하십시오')
  if (!selectedTemplate.value) warnings.push('메시지 Template을 선택하십시오')
  if (selectedTemplate.value && !compatibleTemplateOptions.value.some((item) => Number(item.id) === Number(selectedTemplate.value.id))) {
    warnings.push('메시지 Template과 적용 범위가 일치하지 않습니다')
  }
  if (!selectedChannels.value.length) warnings.push('Notification Channel을 선택하십시오')
  const templateType = selectedTemplate.value?.channelType
  if (templateType && selectedChannels.value.some((item) => item.channelType !== templateType)) {
    warnings.push('Template과 Notification Channel 유형이 일치하지 않습니다')
  }
  return warnings
})

function scopeLabel(value) {
  if (!value) return '범위 미선택'
  if (value === 'all') return '범위 지정 필요'
  return scopeOptions.find((item) => item.value === value)?.label || value
}

function templateScopeLabel(value) {
  return value === 'all' ? '범용 Template' : scopeLabel(value)
}

function eventLabel(value) {
  return eventOptions.find((item) => item.value === value)?.label || value
}

function typeLabel(value) {
  return channelTypeLabels[value] || value || '-'
}

function defaultEventsForScope(scope) {
  if (scope === 'monitor') return ['firing', 'recovered']
  if (scope === 'job') return ['failed', 'waiting_approval', 'rejected']
  if (scope === 'pipeline') return ['success', 'failed', 'waiting_approval', 'rejected']
  return ['success', 'failed']
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    scope: 'monitor',
    events: ['firing', 'recovered'],
    templateId: undefined,
    channelIds: [],
    status: 1,
    description: ''
  })
}

async function loadBaseOptions() {
  optionsLoading.value = true
  try {
    const [templates, channels] = await Promise.all([
      queryNotifyTemplateOptions(),
      queryNotifyChannelOptions()
    ])
    templateOptions.value = Array.isArray(templates) ? templates : []
    channelOptions.value = Array.isArray(channels) ? channels : []
  } catch (error) {
    templateOptions.value = []
    channelOptions.value = []
    ElMessage.error(error?.message || '메시지 Template과 Notification Channel 로드에 실패했습니다')
  } finally {
    optionsLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifyRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function openCreate() {
  isEdit.value = false
  activeStep.value = 0
  resetForm()
  dialogVisible.value = true
  await loadBaseOptions()
}

async function openEdit(row) {
  isEdit.value = true
  activeStep.value = 0
  dialogVisible.value = true
  const [data] = await Promise.all([
    notifyRuleInfo(row.id),
    loadBaseOptions()
  ])
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    scope: data.scope === 'all' ? undefined : (data.scope || undefined),
    events: data.events || [],
    templateId: data.templateId,
    channelIds: data.channelIds || [],
    status: data.status || 1,
    description: data.description || ''
  })
}

function handleScopeChange(value) {
  form.events = defaultEventsForScope(value)
  if (!compatibleTemplateOptions.value.some((item) => Number(item.id) === Number(form.templateId))) {
    form.templateId = undefined
    form.channelIds = []
  }
}

function handleEventsChange(values) {
  const last = values[values.length - 1]
  form.events = last === 'all' ? ['all'] : values.filter((item) => item !== 'all')
}

function handleTemplateChange(value) {
  const template = templateOptions.value.find((item) => Number(item.id) === Number(value))
  const templateScope = template?.scope || 'all'
  if (!template || form.scope !== 'all' || templateScope === 'all') return
  form.scope = templateScope
  form.events = defaultEventsForScope(templateScope)
  ElMessage.info(`Template에 따라 "${scopeLabel(templateScope)}" 시나리오로 전환했습니다`)
}

function channelDisabled(item) {
  return Boolean(selectedTemplate.value && item.channelType !== selectedTemplate.value.channelType)
}

function nextStep() {
  if (activeStep.value === 0) {
    if (!form.name.trim()) return ElMessage.warning('먼저 Rule 이름을 입력하십시오')
    if (!form.events.length) return ElMessage.warning('하나 이상의 트리거 Event를 선택하십시오')
  }
  if (activeStep.value === 1 && routeWarnings.value.length) {
    return ElMessage.warning(routeWarnings.value[0])
  }
  activeStep.value = Math.min(activeStep.value + 1, wizardSteps.length - 1)
}

function previousStep() {
  activeStep.value = Math.max(activeStep.value - 1, 0)
}

async function submit() {
  if (!form.name.trim() || routeWarnings.value.length) {
    ElMessage.warning(routeWarnings.value[0] || 'Rule 이름을 입력하십시오')
    return
  }
  saving.value = true
  try {
    await saveNotifyRule(form)
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleTest(row) {
  await ElMessageBox.confirm(
    `Test는 Rule "${row.name}"에 따라 구성된 Channel로 실제 메시지 1건을 전송합니다. 계속하시겠습니까?`,
    'Notification Rule Test',
    { type: 'warning', confirmButtonText: '전송', cancelButtonText: '취소' }
  )
  const result = await testNotifyRule(row.id)
  ElMessage.success(`Test 메시지가 전송 Queue에 추가되었습니다. 총 ${result?.queued || 0}건`)
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Notification Rule "${row.name}"을(를) 삭제하시겠습니까?`, 'Rule 삭제', { type: 'warning' })
  await deleteNotifyRule(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

function templateName(id) {
  return templateOptions.value.find((item) => Number(item.id) === Number(id))?.name || id || '-'
}

function channelNames(ids) {
  return (ids || [])
    .map((id) => channelOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join(', ') || '-'
}

watch(() => form.templateId, () => {
  if (!selectedTemplate.value) return
  form.channelIds = form.channelIds.filter((id) => {
    const channel = channelOptions.value.find((item) => Number(item.id) === Number(id))
    return channel?.channelType === selectedTemplate.value.channelType
  })
})

onMounted(async () => {
  await loadBaseOptions()
  await loadData()
})
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Notification Rule</h2>
        <p class="page-desc">Business Event, 메시지 Template, Notification Channel을 조합해 재사용 가능한 전송 Routing을 구성합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">Rule 추가</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="Rule 이름 / 설명 검색" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.scope" clearable placeholder="적용 범위" style="width: 160px">
        <el-option v-for="item in filterScopeOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 120px">
        <el-option label="활성화" value="1" />
        <el-option label="비활성화" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="Rule 이름" min-width="180" />
      <el-table-column label="적용 범위" width="130">
        <template #default="{ row }">
          <el-tag :type="row.scope === 'all' ? 'warning' : 'info'" effect="plain">{{ scopeLabel(row.scope) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="트리거 Event" min-width="230">
        <template #default="{ row }">
          <el-tag v-for="item in row.events || []" :key="item" class="tag" effect="plain">{{ eventLabel(item) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="메시지 Template" min-width="170">
        <template #default="{ row }">{{ templateName(row.templateId) }}</template>
      </el-table-column>
      <el-table-column label="Notification Channel" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ channelNames(row.channelIds) }}</template>
      </el-table-column>
      <el-table-column label="상태" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="작업" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">Test</el-button>
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Notification Rule 수정' : 'Notification Rule 추가'" width="880px" destroy-on-close>
      <div class="notify-wizard-steps">
        <button v-for="(step, index) in wizardSteps" :key="step.title" type="button" class="notify-wizard-step" :class="{ active: activeStep === index, done: activeStep > index }" @click="index < activeStep && (activeStep = index)">
          <span>{{ index + 1 }}</span><strong>{{ step.title }}</strong><small>{{ step.description }}</small>
        </button>
      </div>
      <el-form class="notify-wizard-form" label-position="top">
        <template v-if="activeStep === 0">
          <div class="wizard-heading"><h3>어떤 상황에 알림을 보내야 하나요?</h3><p>먼저 시나리오와 트리거 Event를 정의하면 시스템이 적합한 Template을 자동 추천합니다.</p></div>
          <el-form-item label="Rule 이름" required><el-input v-model="form.name" placeholder="예: 운영 Environment Alert Notification" /></el-form-item>
          <el-form-item label="적용 범위">
            <el-select v-model="form.scope" style="width: 100%" @change="handleScopeChange">
              <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="트리거 Event" required>
            <el-checkbox-group v-model="form.events" class="event-choice-grid" @change="handleEventsChange">
              <el-checkbox v-for="item in eventOptions" :key="item.value" :value="item.value" border>{{ item.label }}</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </template>
        <template v-else-if="activeStep === 1">
          <div class="wizard-heading"><h3>누구에게 어떻게 전달할까요?</h3><p>Template과 Channel 유형은 자동으로 검증됩니다. 동일 유형의 Channel은 여러 개를 동시에 선택할 수 있습니다.</p></div>
          <el-form-item label="메시지 Template" required>
          <el-select
            v-model="form.templateId"
            filterable
            :loading="optionsLoading"
            no-data-text="활성화된 메시지 Template이 없습니다. 메시지 Template에서 먼저 생성하거나 활성화하십시오"
            placeholder="Channel 유형과 일치하는 Template을 선택하십시오"
            style="width: 100%"
            @change="handleTemplateChange"
          >
            <el-option v-for="item in compatibleTemplateOptions" :key="item.id" :label="`${item.name} · ${templateScopeLabel(item.scope || 'all')} · ${typeLabel(item.channelType)}`" :value="item.id" />
          </el-select>
          </el-form-item>
          <el-form-item label="Notification Channel" required>
          <el-select v-model="form.channelIds" multiple filterable :loading="optionsLoading" placeholder="동일 유형의 Channel을 여러 개 선택할 수 있습니다" style="width: 100%">
            <el-option v-for="item in channelOptions" :key="item.id" :label="`${item.name} · ${typeLabel(item.channelType)}`" :value="item.id" :disabled="channelDisabled(item)" />
          </el-select>
          </el-form-item>
          <el-alert v-if="routeWarnings.length" :title="routeWarnings.join('; ')" type="warning" :closable="false" show-icon />
        </template>
        <template v-else>
          <div class="wizard-heading"><h3>Notification Routing 확인</h3><p>저장 후 Rule 목록에서 Test 메시지를 전송해 실제 전달 결과를 확인할 수 있습니다.</p></div>
          <div class="route-preview">
            <div class="route-chain">
              <span class="route-node">{{ form.events.length ? form.events.map(eventLabel).join(', ') : 'Event 미선택' }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedTemplate?.name || 'Template 미선택' }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedChannels.length ? selectedChannels.map((item) => item.name).join(', ') : 'Channel 미선택' }}</span>
            </div>
            <div v-if="selectedTemplate" class="route-meta">적용 시나리오: {{ templateScopeLabel(selectedTemplate.scope || 'all') }} · 메시지 형식: {{ typeLabel(selectedTemplate.channelType) }}</div>
            <el-alert v-if="routeWarnings.length" :title="routeWarnings.join('; ')" type="warning" :closable="false" show-icon />
            <el-alert v-else title="Routing 구성이 완전합니다. 저장 후 목록에서 Test 메시지를 전송할 수 있습니다." type="success" :closable="false" show-icon />
          </div>
          <el-form-item label="저장 후 상태" class="wizard-status-field">
            <el-radio-group v-model="form.status"><el-radio :value="1">즉시 활성화</el-radio><el-radio :value="2">비활성 상태로 저장</el-radio></el-radio-group>
          </el-form-item>
          <el-form-item label="설명 (선택)"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="Notification 대상, 사용 시나리오 또는 온콜 요건을 설명합니다" /></el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button v-if="activeStep" @click="previousStep">이전</el-button>
        <el-button v-if="activeStep < wizardSteps.length - 1" type="primary" @click="nextStep">다음</el-button>
        <el-button v-else type="primary" :loading="saving" @click="submit">Rule 저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.tag { margin-right: 6px; margin-bottom: 4px; }
.pager { display: flex; justify-content: flex-end; }
.notify-wizard-steps { display:grid; grid-template-columns:repeat(3, 1fr); gap:8px; margin:0 0 22px; padding:8px; border:1px solid #e1eaf8; border-radius:12px; background:#f7faff; }
.notify-wizard-step { display:grid; grid-template-columns:28px 1fr; column-gap:8px; align-items:center; min-height:54px; padding:8px 10px; color:#8a9ab3; text-align:left; border:0; border-radius:9px; background:transparent; cursor:default; }
.notify-wizard-step span { display:grid; width:26px; height:26px; place-items:center; grid-row:span 2; border:1px solid #ced9ea; border-radius:50%; color:#7d8da6; background:#fff; font-size:12px; font-weight:700; }
.notify-wizard-step strong { color:inherit; font-size:13px; }
.notify-wizard-step small { margin-top:2px; color:inherit; font-size:12px; }
.notify-wizard-step.done { cursor:pointer; color:#4f73c6; }
.notify-wizard-step.active { color:#1f5fce; background:#fff; box-shadow:0 4px 14px rgba(44, 84, 160, .08); }
.notify-wizard-step.active span { border-color:#4f73ff; color:#fff; background:#4f73ff; }
.notify-wizard-form { min-height:330px; }
.wizard-heading { margin:0 0 18px; }
.wizard-heading h3 { margin:0 0 5px; color:#1f2a44; font-size:18px; }
.wizard-heading p { margin:0; color:#7485a7; font-size:13px; line-height:1.6; }
.event-choice-grid { display:grid; grid-template-columns:repeat(3, minmax(0, 1fr)); gap:10px; width:100%; }
.event-choice-grid :deep(.el-checkbox) { display:flex; width:100%; height:38px; margin-right:0; }
.wizard-status-field { margin-top:18px; }
.route-preview { width: 100%; padding: 14px; border: 1px solid #dce6f5; border-radius: 6px; background: #f7faff; }
.route-chain { display: flex; align-items: center; gap: 10px; min-width: 0; margin-bottom: 10px; }
.route-node { min-width: 0; padding: 7px 10px; overflow: hidden; color: #264d85; text-overflow: ellipsis; white-space: nowrap; border: 1px solid #c9dbf5; border-radius: 5px; background: #fff; }
.route-arrow { flex: 0 0 auto; color: #6b8fc7; font-size: 18px; }
.route-meta { margin-bottom: 10px; color: #7282a0; font-size: 13px; }
@media (max-width: 720px) { .notify-wizard-steps, .event-choice-grid { grid-template-columns:1fr; } .notify-wizard-form { min-height:0; } }
</style>
