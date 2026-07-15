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
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const templateOptions = ref([])
const channelOptions = ref([])

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', scope: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  scope: 'all',
  events: ['failed', 'waiting_approval'],
  templateId: undefined,
  channelIds: [],
  status: 1,
  description: ''
})

const scopeOptions = [
  { label: '全部场景', value: 'all' },
  { label: '作业编排', value: 'job' },
  { label: '定时任务', value: 'schedule' },
  { label: '监控告警', value: 'monitor' }
]

const eventOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '等待人工确认', value: 'waiting_approval' },
  { label: '已拒绝', value: 'rejected' },
  { label: '告警触发', value: 'firing' },
  { label: '告警恢复', value: 'recovered' },
  { label: '全部事件', value: 'all' }
]

const channelTypeLabels = {
  dingtalk: '钉钉机器人',
  wecom: '企业微信机器人',
  feishu: '飞书机器人',
  webhook: '自定义 Webhook'
}

const selectedTemplate = computed(() => templateOptions.value.find((item) => Number(item.id) === Number(form.templateId)))
const compatibleTemplateOptions = computed(() => templateOptions.value.filter((item) => {
  const templateScope = item.scope || 'all'
  if (templateScope === 'all') return true
  return form.scope !== 'all' && templateScope === form.scope
}))
const selectedChannels = computed(() => channelOptions.value.filter((item) => form.channelIds.map(Number).includes(Number(item.id))))
const routeWarnings = computed(() => {
  const warnings = []
  if (!form.events.length) warnings.push('至少选择一个触发事件')
  if (!selectedTemplate.value) warnings.push('请选择消息模板')
  if (selectedTemplate.value && !compatibleTemplateOptions.value.some((item) => Number(item.id) === Number(selectedTemplate.value.id))) {
    warnings.push('消息模板与适用范围不一致')
  }
  if (!selectedChannels.value.length) warnings.push('请选择通知媒介')
  const templateType = selectedTemplate.value?.channelType
  if (templateType && selectedChannels.value.some((item) => item.channelType !== templateType)) {
    warnings.push('模板与通知媒介类型不一致')
  }
  return warnings
})

function scopeLabel(value) {
  return scopeOptions.find((item) => item.value === value)?.label || value
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
  return ['success', 'failed']
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    scope: 'all',
    events: ['failed', 'waiting_approval'],
    templateId: undefined,
    channelIds: [],
    status: 1,
    description: ''
  })
}

async function loadBaseOptions() {
  const [templates, channels] = await Promise.all([
    queryNotifyTemplateOptions(),
    queryNotifyChannelOptions()
  ])
  templateOptions.value = templates || []
  channelOptions.value = channels || []
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

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await notifyRuleInfo(row.id)
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    scope: data.scope || 'all',
    events: data.events || [],
    templateId: data.templateId,
    channelIds: data.channelIds || [],
    status: data.status || 1,
    description: data.description || ''
  })
  dialogVisible.value = true
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

function channelDisabled(item) {
  return Boolean(selectedTemplate.value && item.channelType !== selectedTemplate.value.channelType)
}

async function submit() {
  if (!form.name.trim() || routeWarnings.value.length) {
    ElMessage.warning(routeWarnings.value[0] || '请填写规则名称')
    return
  }
  saving.value = true
  try {
    await saveNotifyRule(form)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleTest(row) {
  await ElMessageBox.confirm(
    `测试会按照规则「${row.name}」向已配置媒介发送一条真实消息，是否继续？`,
    '测试通知规则',
    { type: 'warning', confirmButtonText: '确认发送', cancelButtonText: '取消' }
  )
  const result = await testNotifyRule(row.id)
  ElMessage.success(`测试消息已进入发送队列，共 ${result?.queued || 0} 条`)
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除通知规则「${row.name}」吗？`, '删除规则', { type: 'warning' })
  await deleteNotifyRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function templateName(id) {
  return templateOptions.value.find((item) => Number(item.id) === Number(id))?.name || id || '-'
}

function channelNames(ids) {
  return (ids || [])
    .map((id) => channelOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join('、') || '-'
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
        <h2 class="page-title">通知规则</h2>
        <p class="page-desc">将业务事件、消息模板和通知媒介组合为可复用的发送路由。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增规则</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索规则名称 / 描述" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.scope" clearable placeholder="适用范围" style="width: 160px">
        <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="规则名称" min-width="180" />
      <el-table-column label="适用范围" width="130">
        <template #default="{ row }">{{ scopeLabel(row.scope) }}</template>
      </el-table-column>
      <el-table-column label="触发事件" min-width="230">
        <template #default="{ row }">
          <el-tag v-for="item in row.events || []" :key="item" class="tag" effect="plain">{{ eventLabel(item) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="消息模板" min-width="170">
        <template #default="{ row }">{{ templateName(row.templateId) }}</template>
      </el-table-column>
      <el-table-column label="通知媒介" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ channelNames(row.channelIds) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">测试</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑通知规则' : '新增通知规则'" width="880px" destroy-on-close>
      <el-form label-width="110px">
        <el-form-item label="规则名称" required><el-input v-model="form.name" placeholder="例如：生产环境告警通知" /></el-form-item>
        <el-form-item label="适用范围">
          <el-select v-model="form.scope" style="width: 100%" @change="handleScopeChange">
            <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="触发事件" required>
          <el-checkbox-group v-model="form.events" @change="handleEventsChange">
            <el-checkbox v-for="item in eventOptions" :key="item.value" :value="item.value">{{ item.label }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="消息模板" required>
          <el-select v-model="form.templateId" filterable placeholder="选择与媒介类型一致的模板" style="width: 100%">
            <el-option v-for="item in compatibleTemplateOptions" :key="item.id" :label="`${item.name} · ${scopeLabel(item.scope || 'all')} · ${typeLabel(item.channelType)}`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="通知媒介" required>
          <el-select v-model="form.channelIds" multiple filterable placeholder="可选择多个同类型媒介" style="width: 100%">
            <el-option v-for="item in channelOptions" :key="item.id" :label="`${item.name} · ${typeLabel(item.channelType)}`" :value="item.id" :disabled="channelDisabled(item)" />
          </el-select>
        </el-form-item>
        <el-form-item label="路由预览">
          <div class="route-preview">
            <div class="route-chain">
              <span class="route-node">{{ form.events.length ? form.events.map(eventLabel).join('、') : '未选事件' }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedTemplate?.name || '未选模板' }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedChannels.length ? selectedChannels.map((item) => item.name).join('、') : '未选媒介' }}</span>
            </div>
            <div v-if="selectedTemplate" class="route-meta">适用场景：{{ scopeLabel(selectedTemplate.scope || 'all') }} · 消息格式：{{ typeLabel(selectedTemplate.channelType) }}</div>
            <el-alert v-if="routeWarnings.length" :title="routeWarnings.join('；')" type="warning" :closable="false" show-icon />
            <el-alert v-else title="路由配置完整，保存后可在列表中发送测试消息。" type="success" :closable="false" show-icon />
          </div>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="说明通知对象、使用场景或值班要求" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
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
.route-preview { width: 100%; padding: 14px; border: 1px solid #dce6f5; border-radius: 6px; background: #f7faff; }
.route-chain { display: flex; align-items: center; gap: 10px; min-width: 0; margin-bottom: 10px; }
.route-node { min-width: 0; padding: 7px 10px; overflow: hidden; color: #264d85; text-overflow: ellipsis; white-space: nowrap; border: 1px solid #c9dbf5; border-radius: 5px; background: #fff; }
.route-arrow { flex: 0 0 auto; color: #6b8fc7; font-size: 18px; }
.route-meta { margin-bottom: 10px; color: #7282a0; font-size: 13px; }
</style>
