<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorSilenceRule,
  monitorSilenceRuleInfo,
  queryMonitorAlertRuleList,
  queryMonitorSilenceRuleList,
  saveMonitorSilenceRule
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const ruleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })
const timeRange = ref([])

const form = reactive({
  id: undefined,
  name: '',
  matchMode: 'select',
  ruleIds: [],
  ruleNamePattern: '',
  severity: '',
  matchersJson: '{}',
  startsAt: 0,
  endsAt: 0,
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
    matchersJson: '{}',
    startsAt: 0,
    endsAt: 0,
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

function selectedRuleNames(ids = []) {
  if (!ids.length) return '全部规则'
  return ids
    .map((id) => ruleOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join('、')
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
    ElMessage.warning('请输入屏蔽规则名称')
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning('请输入规则名正则')
    return
  }
  saving.value = true
  try {
    await saveMonitorSilenceRule(normalizePayload())
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除屏蔽规则「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorSilenceRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(async () => {
  await loadRuleOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警屏蔽规则</h2>
        <p>按具体告警规则或规则名正则静默告警，命中后只记录事件，不发送通知。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增屏蔽</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索名称 / 规则名" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="名称" min-width="180" />
      <el-table-column label="规则匹配" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">多选：{{ selectedRuleNames(row.ruleIds) }}</span>
          <span v-else>正则：{{ row.ruleNamePattern || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="等级" width="90">
        <template #default="{ row }">{{ row.severity || '全部' }}</template>
      </el-table-column>
      <el-table-column prop="matchersJson" label="Label 匹配" min-width="220" show-overflow-tooltip />
      <el-table-column prop="startsAt" label="开始时间" width="180" />
      <el-table-column prop="endsAt" label="结束时间" width="180" />
      <el-table-column label="状态" width="100">
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑屏蔽规则' : '新增屏蔽规则'" width="820px">
      <el-form label-width="120px">
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="规则匹配">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">下拉多选规则</el-radio-button>
            <el-radio-button label="regex">正则匹配规则名</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="告警规则">
          <el-select v-model="form.ruleIds" multiple filterable clearable placeholder="为空表示全部告警规则" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="规则名正则" required>
          <el-input v-model="form.ruleNamePattern" placeholder="例如：^skzy-sh.* 表示匹配 skzy-sh 开头的所有告警规则" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="form.severity" clearable placeholder="全部等级" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="Label 匹配">
          <el-input v-model="form.matchersJson" type="textarea" :rows="3" placeholder='例如：{"instance":"10.0.0.1","job":"node"}' />
        </el-form-item>
        <el-form-item label="屏蔽时间">
          <el-date-picker v-model="timeRange" type="datetimerange" start-placeholder="开始时间" end-placeholder="结束时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 100%" />
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
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
</style>
