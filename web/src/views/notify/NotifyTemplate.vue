<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyTemplate,
  notifyTemplateInfo,
  queryNotifyTemplateList,
  saveNotifyTemplate
} from '../../api/ops'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', channelType: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  channelType: 'dingtalk',
  title: 'OpsAdmin 通知 - {{targetName}}',
  content: '### {{targetName}}\n\n- 状态：{{status}}\n- 摘要：{{summary}}\n- 时间：{{finishedAt}}\n\n{{detail}}',
  status: 1,
  description: ''
})

const channelTypes = [
  { label: '钉钉机器人', value: 'dingtalk' },
  { label: '企微机器人', value: 'wecom' },
  { label: '飞书机器人', value: 'feishu' },
  { label: '自定义 HTTP Webhook', value: 'webhook' }
]

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    channelType: 'dingtalk',
    title: 'OpsAdmin 通知 - {{targetName}}',
    content: '### {{targetName}}\n\n- 状态：{{status}}\n- 摘要：{{summary}}\n- 时间：{{finishedAt}}\n\n{{detail}}',
    status: 1,
    description: ''
  })
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

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await notifyTemplateInfo(row.id)
  Object.assign(form, data)
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
        <p class="page-desc">按通知媒介维护 Markdown / JSON 模板，支持变量渲染后发送到机器人或 Webhook。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增模板</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索模板名称 / 标题" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable placeholder="媒介类型" style="width: 180px">
        <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="模板名称" min-width="180" />
      <el-table-column label="媒介类型" width="180">
        <template #default="{ row }">{{ channelTypeLabel(row.channelType) }}</template>
      </el-table-column>
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updateTime" label="更新时间" width="180" />
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑消息模板' : '新增消息模板'" width="860px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="模板名称" required><el-input v-model="form.name" /></el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="媒介类型">
              <el-select v-model="form.channelType" style="width: 100%">
                <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="16">
            <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="模板内容" required>
              <el-input v-model="form.content" type="textarea" :rows="12" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-alert type="info" :closable="false" show-icon title="可用变量：{{scope}} {{event}} {{targetName}} {{status}} {{summary}} {{detail}} {{startedAt}} {{finishedAt}} {{historyId}} {{currentStepName}}" />
          </el-col>
          <el-col :span="24">
            <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
          </el-col>
        </el-row>
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
.pager { display: flex; justify-content: flex-end; }
</style>
