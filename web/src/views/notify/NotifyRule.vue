<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyRule,
  notifyRuleInfo,
  queryNotifyChannelOptions,
  queryNotifyRuleList,
  queryNotifyTemplateOptions,
  saveNotifyRule
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
  { label: '定时任务', value: 'schedule' }
]

const eventOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '等待人工确认', value: 'waiting_approval' },
  { label: '已拒绝', value: 'rejected' }
]

function scopeLabel(value) {
  return scopeOptions.find((item) => item.value === value)?.label || value
}

function eventLabel(value) {
  return eventOptions.find((item) => item.value === value)?.label || value
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

async function submit() {
  if (!form.name.trim() || !form.templateId || !form.channelIds.length) {
    ElMessage.warning('请填写规则名称，并选择模板和通知媒介')
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

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除通知规则「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteNotifyRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function templateName(id) {
  return templateOptions.value.find((item) => Number(item.id) === Number(id))?.name || id || '-'
}

function channelNames(ids) {
  const values = ids || []
  return values
    .map((id) => channelOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join('、') || '-'
}

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
        <p class="page-desc">像夜莺的告警规则一样，把触发事件、消息模板和通知媒介组合成可复用规则。</p>
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
      <el-table-column label="适用范围" width="140">
        <template #default="{ row }">{{ scopeLabel(row.scope) }}</template>
      </el-table-column>
      <el-table-column label="触发事件" min-width="220">
        <template #default="{ row }">
          <el-tag v-for="item in row.events || []" :key="item" class="tag">{{ eventLabel(item) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="消息模板" min-width="180">
        <template #default="{ row }">{{ templateName(row.templateId) }}</template>
      </el-table-column>
      <el-table-column label="通知媒介" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ channelNames(row.channelIds) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑通知规则' : '新增通知规则'" width="760px">
      <el-form label-width="110px">
        <el-form-item label="规则名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="适用范围">
          <el-select v-model="form.scope" style="width: 100%">
            <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="触发事件">
          <el-checkbox-group v-model="form.events">
            <el-checkbox v-for="item in eventOptions" :key="item.value" :label="item.value">{{ item.label }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="消息模板" required>
          <el-select v-model="form.templateId" filterable style="width: 100%">
            <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="通知媒介" required>
          <el-select v-model="form.channelIds" multiple filterable style="width: 100%">
            <el-option v-for="item in channelOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
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
.tag { margin-right: 6px; }
.pager { display: flex; justify-content: flex-end; }
</style>
