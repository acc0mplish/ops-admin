<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchInternalRecords, deleteInternalRecord, queryInternalRecords, queryInternalZones, saveInternalRecord } from '../../api/domain'

const route = useRoute()
const router = useRouter()
const zoneId = computed(() => Number(route.params.zoneId))
const loading = ref(false)
const saving = ref(false)
const records = ref([])
const zones = ref([])
const selectedRows = ref([])
const tableRef = ref()
const dialogVisible = ref(false)
const batchDialogVisible = ref(false)
const batchMode = ref('create')
const batchRows = ref([])
const keyword = ref('')
const form = reactive({ id: 0, zoneId: 0, host: '', type: 'A', value: '', ttl: 300, status: 1 })

const zone = computed(() => zones.value.find((item) => item.id === zoneId.value) || {})
const fqdn = computed(() => form.host === '@' ? `${zone.value.name || ''}.` : `${form.host || 'host'}.${zone.value.name || ''}.`)
const selectedCount = computed(() => selectedRows.value.length)

function newBatchRow() {
  return { id: 0, zoneId: zoneId.value, host: '', type: 'A', value: '', ttl: 300, status: 1 }
}

async function load() {
  loading.value = true
  try {
    const [zoneList, recordList] = await Promise.all([
      queryInternalZones({}),
      queryInternalRecords({ zoneId: zoneId.value, keyword: keyword.value })
    ])
    zones.value = zoneList || []
    records.value = recordList || []
    selectedRows.value = []
    await nextTick()
    tableRef.value?.clearSelection()
  } finally {
    loading.value = false
  }
}

function create() {
  Object.assign(form, { id: 0, zoneId: zoneId.value, host: '', type: 'A', value: '', ttl: 300, status: 1 })
  dialogVisible.value = true
}

function edit(row) {
  Object.assign(form, row)
  dialogVisible.value = true
}

async function save() {
  if (!form.host.trim() || !form.value.trim()) {
    ElMessage.warning('请填写主机记录和记录值')
    return
  }
  saving.value = true
  try {
    await saveInternalRecord(form)
    ElMessage.success('解析记录已保存并立即生效')
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除 ${row.host}.${zone.value.name}？删除后解析立即失效。`, '删除解析记录', { type: 'warning' })
  await deleteInternalRecord(row.id)
  ElMessage.success('记录已删除')
  await load()
}

function openBatchCreate() {
  batchMode.value = 'create'
  batchRows.value = [newBatchRow(), newBatchRow()]
  batchDialogVisible.value = true
}

function openBatchUpdate() {
  if (!selectedCount.value) return
  batchMode.value = 'update'
  batchRows.value = selectedRows.value.map((row) => ({ ...row }))
  batchDialogVisible.value = true
}

async function handleBatchCommand(command) {
  if (command === 'create') {
    openBatchCreate()
    return
  }
  if (!selectedCount.value) {
    ElMessage.warning('请先选择需要操作的解析记录')
    return
  }
  if (command === 'update') {
    openBatchUpdate()
    return
  }
  await executeSelected(command)
}

function addBatchRow() {
  batchRows.value.push(newBatchRow())
}

function removeBatchRow(index) {
  if (batchRows.value.length === 1) {
    ElMessage.warning('至少保留一条记录')
    return
  }
  batchRows.value.splice(index, 1)
}

async function submitBatch() {
  const invalidIndex = batchRows.value.findIndex((row) => !String(row.host || '').trim() || !String(row.value || '').trim())
  if (invalidIndex >= 0) {
    ElMessage.warning(`第 ${invalidIndex + 1} 条记录缺少主机记录或记录值`)
    return
  }
  saving.value = true
  try {
    const data = await batchInternalRecords({ zoneId: zoneId.value, action: batchMode.value, records: batchRows.value })
    ElMessage.success(`${batchMode.value === 'create' ? '批量新增' : '批量修改'}完成，共处理 ${data.affected || batchRows.value.length} 条记录`)
    batchDialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function executeSelected(action) {
  if (!selectedCount.value) return
  const labels = { enable: '启用', disable: '禁用', delete: '删除' }
  const label = labels[action]
  await ElMessageBox.confirm(`确认${label}选中的 ${selectedCount.value} 条解析记录？操作完成后 DNS 快照将立即刷新。`, `批量${label}`, { type: action === 'delete' ? 'warning' : 'info' })
  const data = await batchInternalRecords({ zoneId: zoneId.value, action, ids: selectedRows.value.map((row) => row.id) })
  ElMessage.success(`已${label} ${data.affected || selectedCount.value} 条解析记录`)
  await load()
}

onMounted(load)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">AUTHORITATIVE RECORDS</div>
        <h2 class="domain-mono">{{ zone.name || '内网 Zone' }}</h2>
        <p>支持 A 与 CNAME；单条或批量保存后统一原子刷新 DNS 内存快照，无需重启服务。</p>
      </div>
      <div>
        <el-button class="back-zone-button" @click="router.push('/domains/internal')">返回 Zone</el-button>
        <el-dropdown v-permission="'domains:internal:record'" trigger="click" @command="handleBatchCommand">
          <el-button>批量操作<span class="batch-dropdown-caret">⌄</span></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="create">批量新增</el-dropdown-item>
              <el-dropdown-item command="update" :disabled="!selectedCount" divided>批量修改</el-dropdown-item>
              <el-dropdown-item command="disable" :disabled="!selectedCount">批量禁用</el-dropdown-item>
              <el-dropdown-item command="enable" :disabled="!selectedCount">批量启用</el-dropdown-item>
              <el-dropdown-item command="delete" :disabled="!selectedCount" divided class="batch-danger-item">批量删除</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button v-permission="'domains:internal:record'" type="primary" @click="create">新增解析记录</el-button>
      </div>
    </div>

    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters">
        <el-input v-model="keyword" clearable placeholder="搜索主机记录 / 记录值" style="width:280px" @keyup.enter="load" />
        <el-button @click="load">查询</el-button>
      </div>
      <div class="domain-toolbar__actions"><el-button @click="router.push('/domains/query-test')">解析测试</el-button></div>
    </div>

    <div v-if="selectedCount" class="domain-batch-bar" role="status">
      <strong>已选择 {{ selectedCount }} 条解析记录</strong>
      <span class="batch-selection-hint">请从上方“批量操作”菜单执行修改、启停或删除。</span>
    </div>

    <div class="domain-table-wrap">
      <el-table ref="tableRef" v-loading="loading" :data="records" border row-key="id" @selection-change="selectedRows=$event">
        <el-table-column type="selection" width="48" reserve-selection />
        <el-table-column prop="host" label="主机记录" min-width="140"><template #default="{row}"><span class="domain-mono">{{ row.host }}</span></template></el-table-column>
        <el-table-column label="完整域名" min-width="250"><template #default="{row}"><span class="domain-mono">{{ row.host === '@' ? `${zone.name}.` : `${row.host}.${zone.name}.` }}</span></template></el-table-column>
        <el-table-column prop="type" label="类型" width="90" />
        <el-table-column prop="value" label="记录值" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.value }}</span></template></el-table-column>
        <el-table-column prop="ttl" label="TTL" width="90" />
        <el-table-column label="状态" width="100"><template #default="{row}"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="140" fixed="right"><template #default="{row}"><span v-permission="'domains:internal:record'"><el-button link type="primary" @click="edit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></span></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>该 Zone 暂无解析记录</strong><p>可单条新增；如需一次录入多条，请使用右上角“批量操作”。</p><el-button v-permission="'domains:internal:record'" type="primary" @click="create">新增解析记录</el-button></div></template>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑解析记录' : '新增解析记录'" width="min(600px, calc(100vw - 32px))">
      <el-form label-width="96px">
        <el-form-item label="完整域名"><el-input :model-value="fqdn" disabled /></el-form-item>
        <el-form-item label="主机记录" required><el-input v-model="form.host" placeholder="grafana 或 @" /></el-form-item>
        <el-form-item label="记录类型" required><el-radio-group v-model="form.type"><el-radio-button value="A">A</el-radio-button><el-radio-button value="CNAME">CNAME</el-radio-button></el-radio-group></el-form-item>
        <el-form-item label="记录值" required><el-input v-model="form.value" :placeholder="form.type === 'A' ? '192.168.10.20' : 'grafana.ops.internal.'" /><div class="domain-form-tip">{{ form.type === 'A' ? '必须是合法 IPv4 地址。' : '必须填写完整域名，系统会自动规范化为 FQDN 并检测明显循环。' }}</div></el-form-item>
        <el-form-item label="TTL"><el-input-number v-model="form.ttl" :min="1" :max="86400" style="width:100%" /></el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.status" :active-value="1" :inactive-value="2" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存并生效</el-button></template>
    </el-dialog>

    <el-dialog v-model="batchDialogVisible" :title="batchMode === 'create' ? '批量新增解析记录' : `批量修改 ${batchRows.length} 条解析记录`" width="min(1120px, calc(100vw - 32px))" top="6vh">
      <div class="domain-form-tip batch-dialog-tip">所有记录校验通过后才会统一保存并刷新 DNS 快照；任意一条失败时不会产生部分更新。</div>
      <div class="batch-record-table">
        <div class="batch-record-row batch-record-head"><span>主机记录</span><span>类型</span><span>记录值</span><span>TTL</span><span>状态</span><span></span></div>
        <div v-for="(row,index) in batchRows" :key="row.id || index" class="batch-record-row">
          <el-input v-model="row.host" placeholder="app 或 @" />
          <el-select v-model="row.type"><el-option label="A" value="A" /><el-option label="CNAME" value="CNAME" /></el-select>
          <el-input v-model="row.value" :placeholder="row.type === 'A' ? '192.168.10.20' : 'target.internal.'" />
          <el-input-number v-model="row.ttl" :min="1" :max="86400" controls-position="right" style="width:100%" />
          <el-switch v-model="row.status" :active-value="1" :inactive-value="2" inline-prompt active-text="启" inactive-text="禁" />
          <el-button v-if="batchMode === 'create'" link type="danger" @click="removeBatchRow(index)">移除</el-button>
        </div>
      </div>
      <el-button v-if="batchMode === 'create'" class="batch-add-row" @click="addBatchRow">继续添加一行</el-button>
      <template #footer><el-button @click="batchDialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="submitBatch">{{ batchMode === 'create' ? '批量保存并生效' : '保存全部修改' }}</el-button></template>
    </el-dialog>
  </section>
</template>

<style scoped>
.back-zone-button {
  --el-button-bg-color: #ecfdf5;
  --el-button-border-color: #a7f3d0;
  --el-button-text-color: #047857;
  --el-button-hover-bg-color: #d1fae5;
  --el-button-hover-border-color: #6ee7b7;
  --el-button-hover-text-color: #047857;
  --el-button-active-bg-color: #a7f3d0;
  --el-button-active-border-color: #34d399;
  --el-button-active-text-color: #065f46;
}
.batch-dropdown-caret { margin-left: 7px; color: #64748b; font-size: 12px; line-height: 1; }
.batch-selection-hint { color: #64748b; font-size: 12px; }
:global(.batch-danger-item:not(.is-disabled)) { color: #dc2626; }
.batch-dialog-tip { margin-bottom: 14px; padding: 10px 12px; border-radius: 8px; background: #eff6ff; color: #1e40af; }
.batch-record-table { display: grid; gap: 8px; max-height: 56vh; overflow: auto; }
.batch-record-row { display: grid; grid-template-columns: 150px 110px minmax(220px,1fr) 130px 74px 54px; align-items: center; gap: 8px; }
.batch-record-head { padding: 0 4px; color: #64748b; font-size: 12px; font-weight: 650; }
.batch-add-row { margin-top: 12px; }
@media (max-width: 800px) { .batch-record-table { overflow-x: auto; } .batch-record-row { min-width: 820px; } }
</style>
