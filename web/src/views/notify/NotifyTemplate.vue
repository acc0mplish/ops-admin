<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyTemplate,
  notifyTemplateInfo,
  queryNotifyTemplateList,
  saveNotifyTemplate
} from '../../api/ops'
import { queryMonitorAlertEventList } from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const templateDialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const templateCategory = ref('all')
const templateScopeCategory = ref('all')
const previewEventId = ref()
const previewEvents = ref([])
const previewLoading = ref(false)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', channelType: '', scope: '', status: '' })
const form = reactive({ id: undefined, name: '', channelType: 'dingtalk', scope: 'monitor', title: '', content: '', status: 1, description: '' })

const scopeOptions = [
  { label: 'Monitor Alert', value: 'monitor' },
  { label: 'Schedule Task', value: 'schedule' },
  { label: 'Job Orchestration', value: 'job' },
  { label: 'CI/CD Pipeline', value: 'pipeline' }
]
const filterScopeOptions = scopeOptions

const channelTypes = [
  { label: 'DingTalk Bot', value: 'dingtalk', tone: 'ding' },
  { label: 'WeCom Bot', value: 'wecom', tone: 'wecom' },
  { label: 'Feishu Bot', value: 'feishu', tone: 'feishu' },
  { label: '사용자 정의 HTTP Webhook', value: 'webhook', tone: 'webhook' }
]

const commonTemplates = [
  { id: 'ding-alert', scope: 'monitor', channelType: 'dingtalk', name: 'DingTalk - Monitor Alert Notification', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '### <font color={{statusColor}}>【{{severity}}】{{alertName}} -- {{status}}</font>\n\n- Datasource: {{datasourceName}}\n- Instance: {{instance}}\n- 현재 값: {{value}}\n- Alert Threshold: {{threshold}}\n- 트리거 시각: {{startedAt}}\n\n> {{summary}}\n\n{{detail}}', description: 'PromQL과 Log Alert에 사용하며 트리거 중에는 빨강, 복구됨에는 초록으로 표시합니다.' },
  { id: 'ding-schedule', scope: 'schedule', channelType: 'dingtalk', name: 'DingTalk - Schedule Task Notification', title: '【Schedule Task】{{taskName}} -- {{status}}', content: '### <font color={{statusColor}}>【Schedule Task】{{taskName}} -- {{status}}</font>\n\n- Task Type: {{taskType}}\n- Cron: {{cronExpr}}\n- 소요 시간: {{duration}}\n- 완료 시각: {{finishedAt}}\n\n> {{summary}}\n\n{{detail}}', description: 'Schedule Task의 성공/실패 Notification용이며 Task, 소요 시간, 결과를 강조합니다.' },
  { id: 'ding-job', scope: 'job', channelType: 'dingtalk', name: 'DingTalk - Job Orchestration Notification', title: '【Job 알림】{{jobName}} · {{stepName}}', content: '### <font color={{statusColor}}>【Job 알림】{{jobName}} · {{stepName}}</font>\n\n- Notification 유형: {{status}}\n- 실행 번호: #{{jobHistoryId}}\n- 트리거 방식: {{triggerType}}\n- Notification 시각: {{notifyAt}}\n\n> **Notification 요약**\n> {{summary}}\n\n{{detail}}', description: 'Job Orchestration의 메시지 알림 Step용이며 Job, 실행 번호, 현재 Step을 표시합니다.' },
  { id: 'wecom-alert', scope: 'monitor', channelType: 'wecom', name: 'WeCom - Monitor Alert Markdown', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '# <font color="{{statusTone}}">【{{severity}}】{{alertName}} -- {{status}}</font>\n\n> Datasource: {{datasourceName}}\n> Instance: {{instance}}\n> 현재 값: {{value}}\n> Threshold: {{threshold}}\n> 트리거 시각: {{startedAt}}\n\n**Alert 요약**\n{{summary}}\n\n{{detail}}', description: 'WeCom Markdown 형식이며 트리거 중에는 빨강, 복구됨에는 초록으로 표시합니다.' },
  { id: 'wecom-schedule', scope: 'schedule', channelType: 'wecom', name: 'WeCom - Schedule Task Notification', title: '【Schedule Task】{{taskName}} -- {{status}}', content: '# 【Schedule Task】{{taskName}} -- {{status}}\n\n> Task Type: {{taskType}}\n> Cron: {{cronExpr}}\n> 소요 시간: {{duration}}\n> 완료 시각: {{finishedAt}}\n\n**실행 요약**\n{{summary}}\n\n{{detail}}', description: 'WeCom Schedule Task Markdown Notification입니다.' },
  { id: 'wecom-job', scope: 'job', channelType: 'wecom', name: 'WeCom - Job Orchestration Notification', title: '【Job 알림】{{jobName}} · {{stepName}}', content: '# 【Job 알림】{{jobName}} · {{stepName}}\n\n> Notification 유형: <font color="info">{{status}}</font>\n> 실행 번호: #{{jobHistoryId}}\n> 트리거 방식: {{triggerType}}\n> Notification 시각: {{notifyAt}}\n\n**Notification 요약**\n{{summary}}\n\n{{detail}}', description: 'Job Orchestration의 메시지 알림 Step용입니다.' },
  { id: 'feishu-alert', scope: 'monitor', channelType: 'feishu', name: 'Feishu - Alert Card Message', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '**상태:** {{status}}\n\n**Datasource:** {{datasourceName}}\n**Instance:** {{instance}}\n**현재 값:** {{value}}\n**Threshold:** {{threshold}}\n**트리거 시각:** {{startedAt}}\n\n---\n\n**Alert 요약**\n{{summary}}\n\n{{detail}}', description: 'Feishu Interactive Card이며 카드 Header는 트리거/복구 상태에 따라 빨강과 초록을 전환합니다.' },
  { id: 'feishu-schedule', scope: 'schedule', channelType: 'feishu', name: 'Feishu - Schedule Task Notification Card', title: '【Schedule Task】{{taskName}} -- {{status}}', content: '**Task 이름:** {{taskName}}\n**Task Type:** {{taskType}}\n**Cron:** {{cronExpr}}\n**소요 시간:** {{duration}}\n**완료 시각:** {{finishedAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}', description: 'Feishu Schedule Task 실행 결과 Card입니다.' },
  { id: 'feishu-job', scope: 'job', channelType: 'feishu', name: 'Feishu - Job Orchestration Notification Card', title: '【Job 알림】{{jobName}} · {{stepName}}', content: '**Notification 유형:** {{status}}\n\n**Job 이름:** {{jobName}}\n**실행 번호:** #{{jobHistoryId}}\n**현재 Step:** {{stepName}}\n**트리거 방식:** {{triggerType}}\n**Notification 시각:** {{notifyAt}}\n\n---\n\n**Notification 요약**\n{{summary}}\n\n{{detail}}', description: 'Feishu Job Orchestration 메시지 알림 Step 전용 Card입니다.' },
  { id: 'webhook-alert', scope: 'monitor', channelType: 'webhook', name: '범용 Webhook - Alert Notification', title: '{{alertName}}', content: '상태: {{status}}\n요약: {{summary}}\nDatasource: {{datasourceName}}\nInstance: {{instance}}\n현재 값: {{value}}\nThreshold: {{threshold}}\n상세: {{detail}}', description: '범용 HTTP Webhook 텍스트 Content이며 Backend가 구조화된 Event 필드를 보완합니다.' },
  { id: 'webhook-schedule', scope: 'schedule', channelType: 'webhook', name: '범용 Webhook - Schedule Task Notification', title: '【Schedule Task】{{taskName}} -- {{status}}', content: 'Task 이름: {{taskName}}\nTask Type: {{taskType}}\nCron: {{cronExpr}}\n소요 시간: {{duration}}\n완료 시각: {{finishedAt}}\n요약: {{summary}}\n상세: {{detail}}', description: '범용 Webhook Schedule Task 실행 결과 Notification입니다.' },
  { id: 'webhook-job', scope: 'job', channelType: 'webhook', name: '범용 Webhook - Job Orchestration Notification', title: '【Job 알림】{{jobName}} · {{stepName}}', content: 'Notification 유형: {{status}}\nJob 이름: {{jobName}}\n실행 번호: {{jobHistoryId}}\n현재 Step: {{stepName}}\n트리거 방식: {{triggerType}}\nNotification 시각: {{notifyAt}}\n요약: {{summary}}\n상세: {{detail}}', description: '범용 Webhook Job Orchestration 메시지 알림 Step입니다.' },
  { id: 'ding-pipeline', scope: 'pipeline', channelType: 'dingtalk', name: 'DingTalk - CI/CD Pipeline Notification', title: '【{{status}}】{{appName}} · {{stageName}}', content: '### <font color={{statusColor}}>【{{status}}】{{appName}} · {{stageName}}</font>\n\n- Pipeline: {{pipelineName}}\n- 실행 번호: {{pipelineRunId}}\n- Environment: {{env}}\n- Branch: {{branch}}\n- Image Version: {{imageTag}}\n- Notification 시각: {{notifyAt}}\n\n> {{summary}}\n\n{{detail}}', description: 'CI/CD Pipeline Stage 완료, 실패 또는 Manual Approval Notification용입니다.' },
  { id: 'wecom-pipeline', scope: 'pipeline', channelType: 'wecom', name: 'WeCom - CI/CD Pipeline Notification', title: '【{{status}}】{{appName}} · {{stageName}}', content: '# <font color="{{statusTone}}">【{{status}}】{{appName}} · {{stageName}}</font>\n\n> Pipeline: {{pipelineName}}\n> 실행 번호: {{pipelineRunId}}\n> Environment: {{env}}\n> Branch: {{branch}}\n> Image Version: {{imageTag}}\n\n**실행 요약**\n{{summary}}\n\n{{detail}}', description: 'WeCom Markdown 형식의 Pipeline 실행 결과 Notification입니다.' },
  { id: 'feishu-pipeline', scope: 'pipeline', channelType: 'feishu', name: 'Feishu - CI/CD Pipeline Notification Card', title: '【{{status}}】{{appName}} · {{stageName}}', content: '**Pipeline:** {{pipelineName}}\n**실행 번호:** #{{pipelineRunId}}\n**Environment:** {{env}}\n**Branch:** {{branch}}\n**Image Version:** {{imageTag}}\n**Notification 시각:** {{notifyAt}}\n\n---\n\n**실행 요약**\n{{summary}}\n\n{{detail}}', description: 'Feishu Card 스타일이며 Build, Deploy, Stage 결과 Notification에 사용합니다.' },
  { id: 'webhook-pipeline', scope: 'pipeline', channelType: 'webhook', name: '범용 Webhook - CI/CD Pipeline Notification', title: '【{{status}}】{{appName}} · {{stageName}}', content: 'Pipeline: {{pipelineName}}\n실행 번호: {{pipelineRunId}}\nEnvironment: {{env}}\nBranch: {{branch}}\nImage Version: {{imageTag}}\nStage: {{stageName}}\n요약: {{summary}}\n상세: {{detail}}', description: '범용 HTTP Webhook의 Pipeline 구조화 Notification입니다.' }
]

const visibleTemplates = computed(() => commonTemplates.filter((item) =>
  (templateCategory.value === 'all' || item.channelType === templateCategory.value) &&
  (templateScopeCategory.value === 'all' || item.scope === templateScopeCategory.value)
))
const selectedChannel = computed(() => channelTypes.find((item) => item.value === form.channelType) || channelTypes[0])
const scopeVariables = {
  all: ['scope', 'event', 'targetName', 'status', 'summary', 'detail', 'startedAt', 'finishedAt'],
  monitor: ['alertName', 'severity', 'datasourceName', 'instance', 'value', 'threshold'],
  schedule: ['taskName', 'taskType', 'cronExpr', 'duration', 'triggerType'],
  job: ['jobName', 'jobHistoryId', 'stepName', 'stepMessage', 'triggerType', 'notifyAt'],
  pipeline: ['pipelineName', 'pipelineRunId', 'appName', 'env', 'branch', 'imageTag', 'stageName', 'notifyAt']
}
const availableVariables = computed(() => [...scopeVariables.all, ...(scopeVariables[form.scope] || [])])

const previewContext = reactive({
  scope: 'monitor', event: 'firing', targetName: 'Host CPU 사용률 과다', status: '트리거 중', summary: 'node-01 CPU 사용률이 5분 연속 90%를 초과했습니다',
  detail: 'instance="10.0.0.12:9100", job="node-exporter"', startedAt: '2026-07-10 18:30:00', finishedAt: '-',
  alertName: 'Host CPU 사용률 과다', severity: 'P1', datasourceName: '운영 Prometheus', instance: '10.0.0.12:9100', value: '93.42', threshold: '90',
  taskName: '핵심 API Health Check', taskType: 'HTTP Probe', cronExpr: '0 */5 * * * *', duration: '1.8초',
  jobName: '운영 서비스 Rolling Deploy', jobHistoryId: '1024', stepName: 'Deploy 결과 Notification', stepMessage: '모든 Deploy Step이 실행 완료되었습니다', triggerType: '수동 트리거', notifyAt: '2026-07-15 11:05:35',
  statusColor: '#F53F3F', statusTone: 'warning'
})

function renderPreview(value) {
  return String(value || '').replace(/{{\s*([^}]+)\s*}}/g, (_, key) => previewContext[key.trim()] ?? `{{${key}}}`)
}

function statusStyle(status) {
  if (['recovered', 'resolved', 'success'].includes(status)) return { color: '#00B42A', tone: 'info' }
  if (['firing', 'failed'].includes(status)) return { color: '#F53F3F', tone: 'warning' }
  return { color: '#FF7D00', tone: 'comment' }
}

function variableToken(key) {
  return `{{${key}}}`
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    channelType: 'dingtalk',
    scope: 'monitor',
    title: '【{{severity}}】{{alertName}}',
    content: '### {{alertName}}\n\n- 상태: {{status}}\n- 요약: {{summary}}\n- 시각: {{startedAt}}\n\n{{detail}}',
    status: 1,
    description: ''
  })
}

function scopeLabel(value) {
  if (!value) return '시나리오 미선택'
  if (value === 'all') return '시나리오 지정 필요'
  return scopeOptions.find((item) => item.value === value)?.label || value
}

async function preparePreview() {
  previewEventId.value = undefined
  if (form.scope === 'monitor') {
    await loadPreviewEvents()
    return
  }
  const success = statusStyle('success')
  if (form.scope === 'schedule') {
    Object.assign(previewContext, {
      scope: 'schedule', event: 'success', targetName: '핵심 API Health Check', status: '성공',
      summary: 'HTTP Probe가 200을 반환했으며 기대 상태 코드와 일치합니다.', detail: 'GET https://api.example.com/health -> 200',
      startedAt: '2026-07-15 10:00:00', finishedAt: '2026-07-15 10:00:02', taskName: '핵심 API Health Check',
      taskType: 'HTTP Probe', cronExpr: '0 */5 * * * *', duration: '1.8초', triggerType: '스케줄 실행',
      statusColor: success.color, statusTone: success.tone
    })
    return
  }
  if (form.scope === 'job') {
    Object.assign(previewContext, {
      scope: 'job', event: 'notify', targetName: '운영 서비스 Rolling Deploy', status: 'Notification',
      summary: '운영 Deploy가 결과 Notification Step에 진입했습니다.', detail: '모든 Deploy Step이 실행 완료되었습니다. Business Health 상태를 확인하십시오.',
      startedAt: '2026-07-15 10:58:10', finishedAt: '2026-07-15 11:05:35', jobName: '운영 서비스 Rolling Deploy',
      jobHistoryId: '1024', stepName: 'Deploy 결과 Notification', stepMessage: '모든 Deploy Step이 실행 완료되었습니다', triggerType: '수동 트리거',
      notifyAt: '2026-07-15 11:05:35', statusColor: '#3370FF', statusTone: 'info'
    })
    return
  }
  if (form.scope === 'pipeline') {
    Object.assign(previewContext, {
      scope: 'pipeline', event: 'notify', targetName: '운영 Deploy Pipeline', status: 'Notification',
      summary: 'Pipeline이 메시지 알림 Stage에 도달했습니다.', detail: 'Pipeline 실행 기록에서 Stage 실행 상황을 확인하십시오.',
      startedAt: '2026-07-21 10:00:00', finishedAt: '2026-07-21 10:05:35',
      pipelineName: '운영 Deploy Pipeline', pipelineRunId: '1024', appName: '주문 서비스', env: 'prod', branch: 'main', imageTag: 'v20260721.1', stageName: 'Deploy 결과 Notification', notifyAt: '2026-07-21 10:05:35', statusColor: '#3370FF', statusTone: 'info'
    })
    return
  }
  Object.assign(previewContext, { scope: 'all', event: 'notify', targetName: '플랫폼 Notification', status: 'Notification', summary: '범용 Notification입니다.', detail: 'Notification 상세', statusColor: '#3370FF', statusTone: 'info' })
}

function parseLabels(raw) {
  try {
    return JSON.parse(raw || '{}') || {}
  } catch (_) {
    return {}
  }
}

function applyPreviewEvent(id) {
  const event = previewEvents.value.find((item) => Number(item.id) === Number(id))
  if (!event) return
  const labels = parseLabels(event.labelsJson)
  const style = statusStyle(event.status)
  Object.assign(previewContext, {
    scope: 'monitor',
    event: event.status || 'firing',
    targetName: event.ruleName || event.metric || 'Alert Event',
    status: ({ pending: '대기 중', firing: '트리거 중', claimed: '인계됨', silenced: '차단됨', recovered: '복구됨', resolved: '종료됨' }[event.status] || event.status || '-'),
    summary: event.summary || '-',
    detail: event.labelsJson || '-',
    startedAt: event.firstTriggerAt || event.lastTriggerAt || '-',
    finishedAt: event.recoveredAt || '-',
    alertName: event.ruleName || event.metric || 'Alert Event',
    severity: event.severity || '-',
    datasourceName: event.datasourceName || labels.datasource || '-',
    instance: labels.instance || labels.pod || labels.service || '-',
    value: event.currentValue ?? '-',
    threshold: event.threshold ?? '-',
    statusColor: style.color,
    statusTone: style.tone
  })
}

async function loadPreviewEvents() {
  previewLoading.value = true
  try {
    const data = await queryMonitorAlertEventList({ pageNum: 1, pageSize: 100, keyword: '' })
    previewEvents.value = data.list || []
    const first = previewEvents.value.find((item) => ['firing', 'claimed', 'pending'].includes(item.status)) || previewEvents.value[0]
    if (first) {
      previewEventId.value = first.id
      applyPreviewEvent(first.id)
    }
  } finally {
    previewLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifyTemplateList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function openCreate() {
  isEdit.value = false
  resetForm()
  await preparePreview()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  Object.assign(form, await notifyTemplateInfo(row.id))
  if (!form.scope || form.scope === 'all') form.scope = undefined
  await preparePreview()
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateCategory.value = form.channelType || 'all'
  templateScopeCategory.value = form.scope || 'all'
  templateDialogVisible.value = true
}

async function applyTemplate(template) {
  const { id, ...templateForm } = template
  Object.assign(form, { id: undefined, ...templateForm, status: 1 })
  isEdit.value = false
  templateDialogVisible.value = false
  await preparePreview()
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.content.trim()) {
    ElMessage.warning('Template 이름과 Template 내용을 입력하십시오.')
    return
  }
  if (!form.scope || form.scope === 'all') {
    ElMessage.warning('적용 시나리오를 구체적으로 지정하십시오.')
    return
  }
  saving.value = true
  try {
    await saveNotifyTemplate(form)
    ElMessage.success('저장했습니다.')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Template "${row.name}"을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteNotifyTemplate(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

function channelTypeLabel(value) {
  return channelTypes.find((item) => item.value === value)?.label || value
}

watch(() => form.scope, preparePreview)

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">메시지 Template</h2>
        <p class="page-desc">DingTalk, WeCom, Feishu, Webhook용 재사용 가능한 메시지 스타일을 관리합니다. 변수는 전송 시 자동 치환됩니다.</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">자주 쓰는 Template 가져오기</el-button><el-button type="primary" @click="openCreate">Template 추가</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="Template 이름 / 제목 검색" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable placeholder="Channel 유형" style="width: 180px"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select>
      <el-select v-model="query.scope" clearable placeholder="적용 시나리오" style="width: 150px"><el-option v-for="item in filterScopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 120px"><el-option label="활성화" value="1" /><el-option label="비활성화" value="2" /></el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="Template 이름" min-width="180" />
      <el-table-column label="Channel 유형" width="180"><template #default="{ row }"><el-tag effect="light">{{ channelTypeLabel(row.channelType) }}</el-tag></template></el-table-column>
      <el-table-column label="적용 시나리오" width="120"><template #default="{ row }"><el-tag effect="plain" :type="row.scope === 'all' ? 'warning' : 'info'">{{ scopeLabel(row.scope) }}</el-tag></template></el-table-column>
      <el-table-column prop="title" label="제목" min-width="220" show-overflow-tooltip />
      <el-table-column prop="description" label="설명" min-width="240" show-overflow-tooltip />
      <el-table-column label="상태" width="100" align="center"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag></template></el-table-column>
      <el-table-column prop="updateTime" label="수정 시각" width="180" />
      <el-table-column label="작업" width="150" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">수정</el-button><el-button link type="danger" @click="handleDelete(row)">삭제</el-button></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '메시지 Template 수정' : '메시지 Template 추가'" width="1180px" top="5vh">
      <el-form label-width="100px">
        <el-row :gutter="18">
          <el-col :span="8"><el-form-item label="Template 이름" required><el-input v-model="form.name" placeholder="예: 운영 Environment P1 Alert" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Channel 유형"><el-select v-model="form.channelType" style="width: 100%"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="적용 시나리오" required><el-select v-model="form.scope" placeholder="적용 시나리오를 선택하십시오" style="width: 100%"><el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
          <el-col :span="18"><el-form-item label="제목"><el-input v-model="form.title" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="상태"><el-radio-group v-model="form.status"><el-radio :value="1">활성화</el-radio><el-radio :value="2">비활성화</el-radio></el-radio-group></el-form-item></el-col>
        </el-row>
        <div class="template-workbench">
          <section class="editor-pane">
            <div class="pane-title"><strong>Template 내용</strong><el-button link type="primary" @click="openTemplateDialog">자주 쓰는 Template 가져오기</el-button></div>
            <el-input v-model="form.content" class="template-editor" type="textarea" :rows="18" placeholder="Markdown으로 메시지 내용을 작성하십시오" />
            <div class="variable-list"><span>{{ scopeLabel(form.scope) }} 사용 가능 변수</span><code v-for="key in availableVariables" :key="key">{{ variableToken(key) }}</code></div>
          </section>
          <section class="preview-pane" :class="`preview-${selectedChannel.tone}`">
            <div class="pane-title preview-title"><strong>전송 미리보기</strong><div><template v-if="form.scope === 'monitor'"><el-select v-model="previewEventId" :loading="previewLoading" filterable clearable placeholder="Alert Event 선택" style="width: 220px" @change="applyPreviewEvent"><el-option v-for="item in previewEvents" :key="item.id" :label="`${item.ruleName || item.metric} / ${item.status}`" :value="item.id" /></el-select><el-button link type="primary" @click="loadPreviewEvents">새로 고침</el-button></template><el-tag v-else effect="plain">{{ scopeLabel(form.scope) }} 예시 데이터</el-tag></div></div>
            <div v-if="form.channelType === 'feishu'" class="feishu-card">
              <div class="feishu-header" :style="{ background: previewContext.statusColor }"><span>{{ scopeLabel(form.scope) }}</span><strong>{{ renderPreview(form.title) || 'Notification 제목' }}</strong></div>
              <pre>{{ renderPreview(form.content) }}</pre>
              <div class="feishu-footer">Ops Admin</div>
            </div>
            <div v-else-if="form.channelType === 'wecom'" class="wecom-message"><strong>{{ renderPreview(form.title) || 'Notification 제목' }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else-if="form.channelType === 'dingtalk'" class="dingtalk-message"><div class="ding-top">DINGTALK BOT</div><strong>{{ renderPreview(form.title) || 'Notification 제목' }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else class="webhook-message"><span>POST / webhook</span><pre>{{ JSON.stringify({ title: renderPreview(form.title), content: renderPreview(form.content), status: previewContext.status, targetName: previewContext.targetName }, null, 2) }}</pre></div>
          </section>
        </div>
        <el-form-item label="설명" style="margin-top: 18px"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="적용 시나리오와 온콜 사용 방식을 설명하십시오" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">취소</el-button><el-button type="primary" :loading="saving" @click="submit">저장</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" title="자주 쓰는 메시지 Template 가져오기" width="980px" top="8vh">
      <div class="template-import-head"><div><strong>메시지 스타일 선택</strong><span>선택하면 적용 시나리오, Channel 유형, 제목, 내용이 채워집니다.</span></div><div class="import-filters"><el-select v-model="templateScopeCategory" style="width: 150px"><el-option label="전체 시나리오" value="all" /><el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select><el-select v-model="templateCategory" style="width: 190px"><el-option label="전체 Channel" value="all" /><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></div></div>
      <div class="template-card-grid">
        <button v-for="item in visibleTemplates" :key="item.id" type="button" class="import-card" @click="applyTemplate(item)">
          <div class="template-tags"><el-tag size="small" :type="item.channelType === 'feishu' ? 'primary' : item.channelType === 'wecom' ? 'success' : item.channelType === 'dingtalk' ? 'warning' : 'info'">{{ channelTypeLabel(item.channelType) }}</el-tag><el-tag size="small" effect="plain" type="info">{{ scopeLabel(item.scope) }}</el-tag></div>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.title }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">취소</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.header-actions { display: flex; gap: 10px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.template-workbench { display: grid; grid-template-columns: minmax(0, 1.08fr) minmax(360px, .92fr); gap: 18px; }
.editor-pane, .preview-pane { min-height: 500px; border: 1px solid #dbe5f4; border-radius: 8px; overflow: hidden; }
.editor-pane { background: #fff; }
.preview-pane { padding: 14px; background: #f5f7fb; }
.pane-title { display: flex; align-items: center; justify-content: space-between; height: 44px; padding: 0 14px; border-bottom: 1px solid #e3eaf5; color: #263d65; }
.pane-title span { color: #8491a9; font-size: 12px; }
.preview-title > div { display: flex; align-items: center; gap: 6px; }
.template-editor :deep(.el-textarea__inner) { min-height: 370px !important; border: 0; border-radius: 0; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.65; }
.variable-list { display: flex; flex-wrap: wrap; gap: 6px; padding: 12px 14px; border-top: 1px solid #e3eaf5; color: #71809b; font-size: 12px; }
.variable-list span { width: 100%; margin-bottom: 2px; font-weight: 600; }
.variable-list code { padding: 3px 6px; border-radius: 4px; background: #eff3fb; color: #4567c7; }
.preview-pane pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-family: inherit; font-size: 13px; line-height: 1.7; }
.dingtalk-message, .wecom-message, .feishu-card, .webhook-message { margin-top: 20px; overflow: hidden; border-radius: 8px; box-shadow: 0 8px 20px rgba(27, 47, 84, .12); }
.dingtalk-message { padding: 18px; background: #fff; border-left: 4px solid #2f77ff; }
.ding-top { margin-bottom: 14px; color: #2f77ff; font-size: 11px; font-weight: 700; letter-spacing: .8px; }
.dingtalk-message strong, .wecom-message strong { display: block; margin-bottom: 12px; color: #1f2f4e; font-size: 17px; }
.wecom-message { padding: 18px; background: #fff; border-top: 4px solid #1aad19; }
.feishu-card { background: #fff; border: 1px solid #dce5f3; }
.feishu-header { padding: 16px 18px; color: #fff; background: #3370ff; }
.feishu-header span { display: block; opacity: .78; font-size: 12px; }
.feishu-header strong { display: block; margin-top: 6px; font-size: 17px; }
.feishu-card pre { padding: 18px; color: #263d65; }
.feishu-footer { padding: 10px 18px; border-top: 1px solid #edf1f7; color: #97a4b8; font-size: 12px; }
.webhook-message { background: #10182b; color: #d8e4ff; }
.webhook-message span { display: block; padding: 10px 14px; color: #8eb0ff; background: #18233d; font: 12px Consolas, monospace; }
.webhook-message pre { padding: 16px; font-family: Consolas, Monaco, monospace; color: #d8e4ff; }
.template-import-head { display: flex; align-items: center; justify-content: space-between; gap: 18px; margin-bottom: 16px; }
.template-import-head strong { display: block; color: #20355c; }
.template-import-head span { display: block; margin-top: 5px; color: #8491a9; font-size: 13px; }
.import-filters, .template-tags { display: flex; align-items: center; gap: 8px; }
.template-card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; max-height: 520px; overflow: auto; padding-right: 4px; }
.import-card { min-height: 148px; padding: 16px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .2s, box-shadow .2s; }
.import-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.import-card strong { display: block; margin-top: 12px; color: #1d3154; }
.import-card p { min-height: 36px; margin: 7px 0; color: #7282a0; font-size: 13px; line-height: 1.45; }
.import-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 900px) { .template-workbench { grid-template-columns: 1fr; } .template-card-grid { grid-template-columns: 1fr; } }
</style>
