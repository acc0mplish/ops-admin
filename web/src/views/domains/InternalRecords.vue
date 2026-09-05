<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchInternalRecords, deleteInternalRecord, queryInternalRecords, queryInternalZones, saveInternalRecord } from '../../api/domain'
import { dt } from '../../utils/domain-i18n'

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
    ElMessage.warning(dt('warnHostValueInternal'))
    return
  }
  saving.value = true
  try {
    await saveInternalRecord(form)
    ElMessage.success(dt('recordSavedImmediate'))
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(dt('deleteRecordConfirmInternal', { fqdn: `${row.host}.${zone.value.name}` }), dt('deleteInternalRecordTitle'), { type: 'warning' })
  await deleteInternalRecord(row.id)
  ElMessage.success(dt('recordDeleted'))
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
    ElMessage.warning(dt('warnSelectFirst'))
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
    ElMessage.warning(dt('warnMinOneRow'))
    return
  }
  batchRows.value.splice(index, 1)
}

async function submitBatch() {
  const invalidIndex = batchRows.value.findIndex((row) => !String(row.host || '').trim() || !String(row.value || '').trim())
  if (invalidIndex >= 0) {
    ElMessage.warning(dt('batchRowInvalid', { index: invalidIndex + 1 }))
    return
  }
  saving.value = true
  try {
    const data = await batchInternalRecords({ zoneId: zoneId.value, action: batchMode.value, records: batchRows.value })
    ElMessage.success(dt(batchMode.value === 'create' ? 'batchCreateDone' : 'batchUpdateDone', { count: data.affected || batchRows.value.length }))
    batchDialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function executeSelected(action) {
  if (!selectedCount.value) return
  const confirmKey = { enable: 'batchEnableConfirm', disable: 'batchDisableConfirm', delete: 'batchDeleteConfirm' }[action]
  const titleKey = { enable: 'batchEnable', disable: 'batchDisable', delete: 'batchDelete' }[action]
  const doneKey = { enable: 'batchEnableDone', disable: 'batchDisableDone', delete: 'batchDeleteDone' }[action]
  await ElMessageBox.confirm(dt(confirmKey, { count: selectedCount.value }), dt(titleKey), { type: action === 'delete' ? 'warning' : 'info' })
  const data = await batchInternalRecords({ zoneId: zoneId.value, action, ids: selectedRows.value.map((row) => row.id) })
  ElMessage.success(dt(doneKey, { count: data.affected || selectedCount.value }))
  await load()
}

onMounted(load)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">{{ uiT('authoritativeRecords') }}</div>
        <h2 class="domain-mono">{{ zone.name || dt('internalZoneFallback') }}</h2>
        <p>{{ dt('irRecordPageDesc') }}</p>
      </div>
      <div>
        <el-button class="back-zone-button" @click="router.push('/domains/internal')">{{ dt('backToZoneList') }}</el-button>
        <el-dropdown v-permission="'domains:internal:record'" trigger="click" @command="handleBatchCommand">
          <el-button>{{ dt('batchActions') }}<span class="batch-dropdown-caret">⌄</span></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="create">{{ dt('batchCreate') }}</el-dropdown-item>
              <el-dropdown-item command="update" :disabled="!selectedCount" divided>{{ dt('batchUpdate') }}</el-dropdown-item>
              <el-dropdown-item command="disable" :disabled="!selectedCount">{{ dt('batchDisable') }}</el-dropdown-item>
              <el-dropdown-item command="enable" :disabled="!selectedCount">{{ dt('batchEnable') }}</el-dropdown-item>
              <el-dropdown-item command="delete" :disabled="!selectedCount" divided class="batch-danger-item">{{ dt('batchDelete') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button v-permission="'domains:internal:record'" type="primary" @click="create">{{ dt('addInternalRecord') }}</el-button>
      </div>
    </div>

    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters">
        <el-input v-model="keyword" clearable :placeholder="dt('searchHostValueInternal')" style="width:280px" @keyup.enter="load" />
        <el-button @click="load">{{ dt('queryBtn') }}</el-button>
      </div>
      <div class="domain-toolbar__actions"><el-button @click="router.push('/domains/query-test')">{{ dt('queryTestTitle') }}</el-button></div>
    </div>

    <div v-if="selectedCount" class="domain-batch-bar" role="status">
      <strong>{{ dt('selectedCountBar', { count: selectedCount }) }}</strong>
      <span class="batch-selection-hint">{{ dt('batchSelectionHint') }}</span>
    </div>

    <div class="domain-table-wrap">
      <el-table ref="tableRef" v-loading="loading" :data="records" border row-key="id" @selection-change="selectedRows=$event">
        <el-table-column type="selection" width="48" reserve-selection />
        <el-table-column prop="host" :label="dt('hostRecordInternal')" min-width="140"><template #default="{row}"><span class="domain-mono">{{ row.host }}</span></template></el-table-column>
        <el-table-column :label="dt('fullDomain')" min-width="250"><template #default="{row}"><span class="domain-mono">{{ row.host === '@' ? `${zone.name}.` : `${row.host}.${zone.name}.` }}</span></template></el-table-column>
        <el-table-column prop="type" :label="dt('type')" width="90" />
        <el-table-column prop="value" :label="dt('recordValueInternal')" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.value }}</span></template></el-table-column>
        <el-table-column prop="ttl" label="TTL" width="90" />
        <el-table-column :label="dt('status')" width="100"><template #default="{row}"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? dt('stateEnabled') : dt('stateDisabled') }}</el-tag></template></el-table-column>
        <el-table-column :label="dt('actions')" width="140" fixed="right"><template #default="{row}"><span v-permission="'domains:internal:record'"><el-button link type="primary" @click="edit(row)">{{ dt('edit') }}</el-button><el-button link type="danger" @click="remove(row)">{{ dt('delete') }}</el-button></span></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>{{ dt('emptyZoneTitle') }}</strong><p>{{ dt('emptyZoneDesc') }}</p><el-button v-permission="'domains:internal:record'" type="primary" @click="create">{{ dt('addInternalRecord') }}</el-button></div></template>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? dt('editInternalRecord') : dt('addInternalRecord')" width="min(600px, calc(100vw - 32px))">
      <el-form label-width="96px">
        <el-form-item :label="dt('fullDomain')"><el-input :model-value="fqdn" disabled /></el-form-item>
        <el-form-item :label="dt('hostRecordInternal')" required><el-input v-model="form.host" :placeholder="dt('hostPlaceholderInternal')" /></el-form-item>
        <el-form-item :label="dt('recordTypeInternal')" required><el-radio-group v-model="form.type"><el-radio-button value="A">A</el-radio-button><el-radio-button value="CNAME">CNAME</el-radio-button></el-radio-group></el-form-item>
        <el-form-item :label="dt('recordValueInternal')" required><el-input v-model="form.value" :placeholder="form.type === 'A' ? '192.168.10.20' : 'grafana.ops.internal.'" /><div class="domain-form-tip">{{ form.type === 'A' ? dt('tipIpv4') : dt('tipFqdn') }}</div></el-form-item>
        <el-form-item label="TTL"><el-input-number v-model="form.ttl" :min="1" :max="86400" style="width:100%" /></el-form-item>
        <el-form-item :label="dt('status')"><el-switch v-model="form.status" :active-value="1" :inactive-value="2" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">{{ dt('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ dt('saveAndApply') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="batchDialogVisible" :title="batchMode === 'create' ? dt('batchAddDialogTitle') : dt('batchEditDialogTitle', { count: batchRows.length })" width="min(1120px, calc(100vw - 32px))" top="6vh">
      <div class="domain-form-tip batch-dialog-tip">{{ dt('batchAllOrNothing') }}</div>
      <div class="batch-record-table">
        <div class="batch-record-row batch-record-head"><span>{{ dt('hostRecordInternal') }}</span><span>{{ dt('type') }}</span><span>{{ dt('recordValueInternal') }}</span><span>TTL</span><span>{{ dt('status') }}</span><span></span></div>
        <div v-for="(row,index) in batchRows" :key="row.id || index" class="batch-record-row">
          <el-input v-model="row.host" :placeholder="dt('batchHostPlaceholder')" />
          <el-select v-model="row.type"><el-option label="A" value="A" /><el-option label="CNAME" value="CNAME" /></el-select>
          <el-input v-model="row.value" :placeholder="row.type === 'A' ? '192.168.10.20' : 'target.internal.'" />
          <el-input-number v-model="row.ttl" :min="1" :max="86400" controls-position="right" style="width:100%" />
          <el-switch v-model="row.status" :active-value="1" :inactive-value="2" inline-prompt :active-text="dt('enabled')" :inactive-text="dt('disabled')" />
          <el-button v-if="batchMode === 'create'" link type="danger" @click="removeBatchRow(index)">{{ dt('removeRow') }}</el-button>
        </div>
      </div>
      <el-button v-if="batchMode === 'create'" class="batch-add-row" @click="addBatchRow">{{ dt('addRow') }}</el-button>
      <template #footer><el-button @click="batchDialogVisible=false">{{ dt('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submitBatch">{{ batchMode === 'create' ? dt('batchSaveApply') : dt('saveAllChanges') }}</el-button></template>
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
