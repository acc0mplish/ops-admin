<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
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
const previewEventId = ref()
const previewEvents = ref([])
const previewLoading = ref(false)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', channelType: '', status: '' })
const form = reactive({ id: undefined, name: '', channelType: 'dingtalk', title: '', content: '', status: 1, description: '' })

const channelTypes = [
  { label: '钉钉机器人', value: 'dingtalk', tone: 'ding' },
  { label: '企微机器人', value: 'wecom', tone: 'wecom' },
  { label: '飞书机器人', value: 'feishu', tone: 'feishu' },
  { label: '自定义 HTTP Webhook', value: 'webhook', tone: 'webhook' }
]

const commonTemplates = [
  { id: 'ding-alert', channelType: 'dingtalk', name: '钉钉 - 监控告警通知', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '### <font color={{statusColor}}>【{{severity}}】{{alertName}} -- {{status}}</font>\n\n- 数据源：{{datasourceName}}\n- 实例：{{instance}}\n- 当前值：{{value}}\n- 告警阈值：{{threshold}}\n- 触发时间：{{startedAt}}\n\n> {{summary}}\n\n{{detail}}', description: '适用于 PromQL 与日志告警，触发中红色、已恢复绿色。' },
  { id: 'ding-job', channelType: 'dingtalk', name: '钉钉 - 作业执行结果', title: '作业{{status}}：{{targetName}}', content: '### 作业执行{{status}}\n\n- 作业：{{targetName}}\n- 事件：{{event}}\n- 开始时间：{{startedAt}}\n- 结束时间：{{finishedAt}}\n\n> {{summary}}\n\n{{detail}}', description: '用于作业编排、快速执行和定时任务通知。' },
  { id: 'wecom-alert', channelType: 'wecom', name: '企微 - 监控告警 Markdown', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '# <font color="{{statusTone}}">【{{severity}}】{{alertName}} -- {{status}}</font>\n\n> 数据源：{{datasourceName}}\n> 实例：{{instance}}\n> 当前值：{{value}}\n> 阈值：{{threshold}}\n> 触发时间：{{startedAt}}\n\n**告警摘要**\n{{summary}}\n\n{{detail}}', description: '企业微信 Markdown 格式，触发中红色、已恢复绿色。' },
  { id: 'wecom-job', channelType: 'wecom', name: '企微 - 发布与作业通知', title: '{{targetName}} {{status}}', content: '# {{targetName}}\n\n> 执行状态：<font color="info">{{status}}</font>\n> 触发事件：{{event}}\n> 开始时间：{{startedAt}}\n> 完成时间：{{finishedAt}}\n\n**执行摘要**\n{{summary}}\n\n{{detail}}', description: '用于应用发布、作业和定时任务执行结果。' },
  { id: 'feishu-alert', channelType: 'feishu', name: '飞书 - 告警卡片消息', title: '【{{severity}}】{{alertName}} -- {{status}}', content: '**状态：** {{status}}\n\n**数据源：** {{datasourceName}}\n**实例：** {{instance}}\n**当前值：** {{value}}\n**阈值：** {{threshold}}\n**触发时间：** {{startedAt}}\n\n---\n\n**告警摘要**\n{{summary}}\n\n{{detail}}', description: '飞书 interactive card，卡片头会随触发/恢复状态切换红绿。' },
  { id: 'feishu-job', channelType: 'feishu', name: '飞书 - 作业结果卡片', title: '作业{{status}}：{{targetName}}', content: '**执行状态：** {{status}}\n\n**作业名称：** {{targetName}}\n**触发事件：** {{event}}\n**开始时间：** {{startedAt}}\n**结束时间：** {{finishedAt}}\n\n---\n\n{{summary}}\n\n{{detail}}', description: '飞书作业、流水线与发布结果通知卡片。' },
  { id: 'webhook-alert', channelType: 'webhook', name: '通用 Webhook - 告警通知', title: '{{alertName}}', content: '状态：{{status}}\n摘要：{{summary}}\n数据源：{{datasourceName}}\n实例：{{instance}}\n当前值：{{value}}\n阈值：{{threshold}}\n详情：{{detail}}', description: '通用 HTTP Webhook 文本内容，后端会补充结构化事件字段。' },
  { id: 'webhook-job', channelType: 'webhook', name: '通用 Webhook - 作业通知', title: '{{targetName}} {{status}}', content: '事件：{{event}}\n状态：{{status}}\n目标：{{targetName}}\n开始：{{startedAt}}\n结束：{{finishedAt}}\n摘要：{{summary}}\n详情：{{detail}}', description: '通用 Webhook 作业和发布执行结果。' }
]

const visibleTemplates = computed(() => commonTemplates.filter((item) => templateCategory.value === 'all' || item.channelType === templateCategory.value))
const selectedChannel = computed(() => channelTypes.find((item) => item.value === form.channelType) || channelTypes[0])

const previewContext = reactive({
  scope: 'monitor', event: 'firing', targetName: '主机 CPU 使用率过高', status: '触发中', summary: 'node-01 CPU 使用率连续 5 分钟超过 90%',
  detail: 'instance="10.0.0.12:9100", job="node-exporter"', startedAt: '2026-07-10 18:30:00', finishedAt: '-',
  alertName: '主机 CPU 使用率过高', severity: 'P1', datasourceName: '生产 Prometheus', instance: '10.0.0.12:9100', value: '93.42', threshold: '90', statusColor: '#F53F3F', statusTone: 'warning'
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
    title: '【{{severity}}】{{alertName}}',
    content: '### {{alertName}}\n\n- 状态：{{status}}\n- 摘要：{{summary}}\n- 时间：{{startedAt}}\n\n{{detail}}',
    status: 1,
    description: ''
  })
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
    targetName: event.ruleName || event.metric || '告警事件',
    status: ({ pending: '等待持续', firing: '触发中', claimed: '已认领', silenced: '已屏蔽', recovered: '已恢复', resolved: '已关闭' }[event.status] || event.status || '-'),
    summary: event.summary || '-',
    detail: event.labelsJson || '-',
    startedAt: event.firstTriggerAt || event.lastTriggerAt || '-',
    finishedAt: event.recoveredAt || '-',
    alertName: event.ruleName || event.metric || '告警事件',
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
  await loadPreviewEvents()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  Object.assign(form, await notifyTemplateInfo(row.id))
  await loadPreviewEvents()
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateCategory.value = form.channelType || 'all'
  templateDialogVisible.value = true
}

async function applyTemplate(template) {
  const { id, ...templateForm } = template
  Object.assign(form, { id: undefined, ...templateForm, status: 1 })
  isEdit.value = false
  templateDialogVisible.value = false
  await loadPreviewEvents()
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.content.trim()) {
    ElMessage.warning('请填写模板名称和模板内容')
    return
  }
  saving.value = true
  try {
    await saveNotifyTemplate(form)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除模板「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteNotifyTemplate(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function channelTypeLabel(value) {
  return channelTypes.find((item) => item.value === value)?.label || value
}

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">消息模板</h2>
        <p class="page-desc">为钉钉、企业微信、飞书与 Webhook 维护可复用消息样式，变量将在发送时自动替换。</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">导入常用模板</el-button><el-button type="primary" @click="openCreate">新增模板</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索模板名称 / 标题" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable placeholder="媒介类型" style="width: 180px"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px"><el-option label="启用" value="1" /><el-option label="禁用" value="2" /></el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="模板名称" min-width="180" />
      <el-table-column label="媒介类型" width="180"><template #default="{ row }"><el-tag effect="light">{{ channelTypeLabel(row.channelType) }}</el-tag></template></el-table-column>
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column prop="description" label="说明" min-width="240" show-overflow-tooltip />
      <el-table-column label="状态" width="100" align="center"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column>
      <el-table-column prop="updateTime" label="更新时间" width="180" />
      <el-table-column label="操作" width="150" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="handleDelete(row)">删除</el-button></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑消息模板' : '新增消息模板'" width="1180px" top="5vh">
      <el-form label-width="100px">
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="模板名称" required><el-input v-model="form.name" placeholder="例如：生产环境 P1 告警" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="媒介类型"><el-select v-model="form.channelType" style="width: 100%"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
          <el-col :span="18"><el-form-item label="标题"><el-input v-model="form.title" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio :value="1">启用</el-radio><el-radio :value="2">禁用</el-radio></el-radio-group></el-form-item></el-col>
        </el-row>
        <div class="template-workbench">
          <section class="editor-pane">
            <div class="pane-title"><strong>模板内容</strong><el-button link type="primary" @click="openTemplateDialog">导入常用模板</el-button></div>
            <el-input v-model="form.content" class="template-editor" type="textarea" :rows="18" placeholder="使用 Markdown 编写消息内容" />
            <div class="variable-list"><span>可用变量</span><code v-for="key in ['scope','event','targetName','status','summary','detail','startedAt','finishedAt','alertName','severity','datasourceName','instance','value','threshold']" :key="key">{{ variableToken(key) }}</code></div>
          </section>
          <section class="preview-pane" :class="`preview-${selectedChannel.tone}`">
            <div class="pane-title preview-title"><strong>发送预览</strong><div><el-select v-model="previewEventId" :loading="previewLoading" filterable clearable placeholder="选择告警事件" style="width: 220px" @change="applyPreviewEvent"><el-option v-for="item in previewEvents" :key="item.id" :label="`${item.ruleName || item.metric} / ${item.status}`" :value="item.id" /></el-select><el-button link type="primary" @click="loadPreviewEvents">刷新</el-button></div></div>
            <div v-if="form.channelType === 'feishu'" class="feishu-card">
              <div class="feishu-header" :style="{ background: previewContext.statusColor }"><span>告警通知</span><strong>{{ renderPreview(form.title) || '通知标题' }}</strong></div>
              <pre>{{ renderPreview(form.content) }}</pre>
              <div class="feishu-footer">Ops Admin</div>
            </div>
            <div v-else-if="form.channelType === 'wecom'" class="wecom-message"><strong>{{ renderPreview(form.title) || '通知标题' }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else-if="form.channelType === 'dingtalk'" class="dingtalk-message"><div class="ding-top">DINGTALK BOT</div><strong>{{ renderPreview(form.title) || '通知标题' }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else class="webhook-message"><span>POST / webhook</span><pre>{{ JSON.stringify({ title: renderPreview(form.title), content: renderPreview(form.content), status: previewContext.status, targetName: previewContext.targetName }, null, 2) }}</pre></div>
          </section>
        </div>
        <el-form-item label="描述" style="margin-top: 18px"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="说明适用场景与值班使用方式" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" title="导入常用消息模板" width="980px" top="8vh">
      <div class="template-import-head"><div><strong>选择消息样式</strong><span>选择后会带入媒介类型、标题、内容与说明。</span></div><el-select v-model="templateCategory" style="width: 190px"><el-option label="全部模板" value="all" /><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></div>
      <div class="template-card-grid">
        <button v-for="item in visibleTemplates" :key="item.id" type="button" class="import-card" @click="applyTemplate(item)">
          <div><el-tag size="small" :type="item.channelType === 'feishu' ? 'primary' : item.channelType === 'wecom' ? 'success' : item.channelType === 'dingtalk' ? 'warning' : 'info'">{{ channelTypeLabel(item.channelType) }}</el-tag></div>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.title }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">取消</el-button></template>
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
.template-card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; max-height: 520px; overflow: auto; padding-right: 4px; }
.import-card { min-height: 148px; padding: 16px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .2s, box-shadow .2s; }
.import-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.import-card strong { display: block; margin-top: 12px; color: #1d3154; }
.import-card p { min-height: 36px; margin: 7px 0; color: #7282a0; font-size: 13px; line-height: 1.45; }
.import-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 900px) { .template-workbench { grid-template-columns: 1fr; } .template-card-grid { grid-template-columns: 1fr; } }
</style>
