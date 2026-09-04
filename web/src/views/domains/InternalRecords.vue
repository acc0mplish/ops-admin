<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
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
    ElMessage.warning('Host 레코드와 레코드 값을 입력하십시오.')
    return
  }
  saving.value = true
  try {
    await saveInternalRecord(form)
    ElMessage.success('DNS 레코드를 저장했으며 즉시 적용됩니다.')
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`${row.host}.${zone.value.name}을(를) 삭제하시겠습니까? 삭제 시 DNS 레코드는 즉시 무효화됩니다.`, 'DNS 레코드 삭제', { type: 'warning' })
  await deleteInternalRecord(row.id)
  ElMessage.success('레코드를 삭제했습니다.')
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
    ElMessage.warning('먼저 처리할 DNS 레코드를 선택하십시오.')
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
    ElMessage.warning('레코드는 최소 1개 이상 유지해야 합니다.')
    return
  }
  batchRows.value.splice(index, 1)
}

async function submitBatch() {
  const invalidIndex = batchRows.value.findIndex((row) => !String(row.host || '').trim() || !String(row.value || '').trim())
  if (invalidIndex >= 0) {
    ElMessage.warning(`${invalidIndex + 1}번째 레코드에 Host 레코드 또는 레코드 값이 없습니다.`)
    return
  }
  saving.value = true
  try {
    const data = await batchInternalRecords({ zoneId: zoneId.value, action: batchMode.value, records: batchRows.value })
    ElMessage.success(`${batchMode.value === 'create' ? '일괄 추가' : '일괄 수정'} 완료: 총 ${data.affected || batchRows.value.length}건의 레코드를 처리했습니다.`)
    batchDialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function executeSelected(action) {
  if (!selectedCount.value) return
  const labels = { enable: '활성화', disable: '비활성화', delete: '삭제' }
  const label = labels[action]
  await ElMessageBox.confirm(`선택한 ${selectedCount.value}개 DNS 레코드를 ${label}하시겠습니까? 완료 후 DNS Snapshot이 즉시 새로 고쳐집니다.`, `일괄 ${label}`, { type: action === 'delete' ? 'warning' : 'info' })
  const data = await batchInternalRecords({ zoneId: zoneId.value, action, ids: selectedRows.value.map((row) => row.id) })
  ElMessage.success(`DNS 레코드 ${data.affected || selectedCount.value}건을 ${label}했습니다.`)
  await load()
}

onMounted(load)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div>
        <div class="domain-eyebrow">{{ uiT('authoritativeRecords') }}</div>
        <h2 class="domain-mono">{{ zone.name || '내부 Zone' }}</h2>
        <p>A와 CNAME을 지원합니다. 단건 또는 일괄 저장 후 DNS 메모리 Snapshot을 원자적으로 새로 고치며 Service 재시작이 필요 없습니다.</p>
      </div>
      <div>
        <el-button class="back-zone-button" @click="router.push('/domains/internal')">Zone 목록으로 돌아가기</el-button>
        <el-dropdown v-permission="'domains:internal:record'" trigger="click" @command="handleBatchCommand">
          <el-button>일괄 작업<span class="batch-dropdown-caret">⌄</span></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="create">일괄 추가</el-dropdown-item>
              <el-dropdown-item command="update" :disabled="!selectedCount" divided>일괄 수정</el-dropdown-item>
              <el-dropdown-item command="disable" :disabled="!selectedCount">일괄 비활성화</el-dropdown-item>
              <el-dropdown-item command="enable" :disabled="!selectedCount">일괄 활성화</el-dropdown-item>
              <el-dropdown-item command="delete" :disabled="!selectedCount" divided class="batch-danger-item">일괄 삭제</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button v-permission="'domains:internal:record'" type="primary" @click="create">DNS 레코드 추가</el-button>
      </div>
    </div>

    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters">
        <el-input v-model="keyword" clearable placeholder="Host 레코드 / 레코드 값 검색" style="width:280px" @keyup.enter="load" />
        <el-button @click="load">조회</el-button>
      </div>
      <div class="domain-toolbar__actions"><el-button @click="router.push('/domains/query-test')">DNS 조회 테스트</el-button></div>
    </div>

    <div v-if="selectedCount" class="domain-batch-bar" role="status">
      <strong>DNS 레코드 {{ selectedCount }}개를 선택했습니다</strong>
      <span class="batch-selection-hint">위의 “일괄 작업” 메뉴에서 수정, 활성화/비활성화 또는 삭제를 실행하십시오.</span>
    </div>

    <div class="domain-table-wrap">
      <el-table ref="tableRef" v-loading="loading" :data="records" border row-key="id" @selection-change="selectedRows=$event">
        <el-table-column type="selection" width="48" reserve-selection />
        <el-table-column prop="host" label="Host 레코드" min-width="140"><template #default="{row}"><span class="domain-mono">{{ row.host }}</span></template></el-table-column>
        <el-table-column label="전체 Domain" min-width="250"><template #default="{row}"><span class="domain-mono">{{ row.host === '@' ? `${zone.name}.` : `${row.host}.${zone.name}.` }}</span></template></el-table-column>
        <el-table-column prop="type" label="유형" width="90" />
        <el-table-column prop="value" label="레코드 값" min-width="220"><template #default="{row}"><span class="domain-mono">{{ row.value }}</span></template></el-table-column>
        <el-table-column prop="ttl" label="TTL" width="90" />
        <el-table-column label="상태" width="100"><template #default="{row}"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag></template></el-table-column>
        <el-table-column label="작업" width="140" fixed="right"><template #default="{row}"><span v-permission="'domains:internal:record'"><el-button link type="primary" @click="edit(row)">수정</el-button><el-button link type="danger" @click="remove(row)">삭제</el-button></span></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>이 Zone에는 DNS 레코드가 없습니다</strong><p>단건으로 추가할 수 있습니다. 한 번에 여러 건을 입력하려면 오른쪽 위의 “일괄 작업”을 사용하십시오.</p><el-button v-permission="'domains:internal:record'" type="primary" @click="create">DNS 레코드 추가</el-button></div></template>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? 'DNS 레코드 수정' : 'DNS 레코드 추가'" width="min(600px, calc(100vw - 32px))">
      <el-form label-width="96px">
        <el-form-item label="전체 Domain"><el-input :model-value="fqdn" disabled /></el-form-item>
        <el-form-item label="Host 레코드" required><el-input v-model="form.host" placeholder="grafana 또는 @" /></el-form-item>
        <el-form-item label="레코드 유형" required><el-radio-group v-model="form.type"><el-radio-button value="A">A</el-radio-button><el-radio-button value="CNAME">CNAME</el-radio-button></el-radio-group></el-form-item>
        <el-form-item label="레코드 값" required><el-input v-model="form.value" :placeholder="form.type === 'A' ? '192.168.10.20' : 'grafana.ops.internal.'" /><div class="domain-form-tip">{{ form.type === 'A' ? '올바른 IPv4 Address여야 합니다.' : '전체 Domain을 입력해야 합니다. 시스템이 자동으로 FQDN으로 정규화하고 명백한 순환을 검사합니다.' }}</div></el-form-item>
        <el-form-item label="TTL"><el-input-number v-model="form.ttl" :min="1" :max="86400" style="width:100%" /></el-form-item>
        <el-form-item label="상태"><el-switch v-model="form.status" :active-value="1" :inactive-value="2" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">취소</el-button><el-button type="primary" :loading="saving" @click="save">저장 후 적용</el-button></template>
    </el-dialog>

    <el-dialog v-model="batchDialogVisible" :title="batchMode === 'create' ? 'DNS 레코드 일괄 추가' : `DNS 레코드 ${batchRows.length}건 일괄 수정`" width="min(1120px, calc(100vw - 32px))" top="6vh">
      <div class="domain-form-tip batch-dialog-tip">모든 레코드가 검증을 통과하면 한꺼번에 저장되고 DNS Snapshot을 새로 고칩니다. 하나라도 실패하면 부분 업데이트가 발생하지 않습니다.</div>
      <div class="batch-record-table">
        <div class="batch-record-row batch-record-head"><span>Host 레코드</span><span>유형</span><span>레코드 값</span><span>TTL</span><span>상태</span><span></span></div>
        <div v-for="(row,index) in batchRows" :key="row.id || index" class="batch-record-row">
          <el-input v-model="row.host" placeholder="app 또는 @" />
          <el-select v-model="row.type"><el-option label="A" value="A" /><el-option label="CNAME" value="CNAME" /></el-select>
          <el-input v-model="row.value" :placeholder="row.type === 'A' ? '192.168.10.20' : 'target.internal.'" />
          <el-input-number v-model="row.ttl" :min="1" :max="86400" controls-position="right" style="width:100%" />
          <el-switch v-model="row.status" :active-value="1" :inactive-value="2" inline-prompt active-text="활성" inactive-text="비활성" />
          <el-button v-if="batchMode === 'create'" link type="danger" @click="removeBatchRow(index)">제거</el-button>
        </div>
      </div>
      <el-button v-if="batchMode === 'create'" class="batch-add-row" @click="addBatchRow">행 추가</el-button>
      <template #footer><el-button @click="batchDialogVisible=false">취소</el-button><el-button type="primary" :loading="saving" @click="submitBatch">{{ batchMode === 'create' ? '일괄 저장 후 적용' : '전체 수정 사항 저장' }}</el-button></template>
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
