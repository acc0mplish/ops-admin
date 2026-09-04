<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorAlertTemplate,
  deleteMonitorAlertTemplateGroup,
  exportPrometheusAlertTemplates,
  monitorAlertTemplateInfo,
  importPrometheusAlertTemplates,
  parsePrometheusAlertTemplates,
  queryMonitorAlertTemplateGroups,
  queryMonitorAlertTemplateList,
  saveMonitorAlertTemplate,
  saveMonitorAlertTemplateGroup,
} from '../../api/monitor'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const detailVisible = ref(false)
const groupDialogVisible = ref(false)
const isEdit = ref(false)
const importVisible = ref(false)
const importStage = ref('paste')
const importParsing = ref(false)
const importSaving = ref(false)
const importTableRef = ref()
const templateTableRef = ref()
const importRows = ref([])
const importSelection = ref([])
const importGroupId = ref()
const duplicateStrategy = ref('skip')
const importContent = ref('')
const importError = ref('')
const exportSelection = ref([])
const exportLoading = ref(false)
const rows = ref([])
const total = ref(0)
const templateTotal = ref(0)
const groups = ref([])
const selectedGroupId = ref(0)
const detail = ref({})
const groupForm = reactive({ id: undefined, parentId: 0, name: '' })
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', datasourceType: '', source: '', groupId: '' })
const form = reactive({ id: undefined, groupId: undefined, name: '', datasourceType: 'prometheus', queryText: '', comparator: '>', threshold: 90, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', labelsJson: '{}', annotationsJson: '{}', description: '', status: 1 })

const groupTree = computed(() => {
  const map = new Map(groups.value.map((item) => [item.id, { ...item, children: [] }]))
  const roots = []
  map.forEach((item) => {
    const parent = map.get(item.parentId)
    if (parent) parent.children.push(item)
    else roots.push(item)
  })
  const decorate = (item) => {
    item.children = item.children.map(decorate)
    item.totalCount = Number(item.count || 0) + item.children.reduce((sum, child) => sum + child.totalCount, 0)
    return item
  }
  return roots.map(decorate)
})

const leafGroupIds = computed(() => {
  const parents = new Set(groups.value.filter((item) => item.parentId).map((item) => item.parentId))
  return groups.value.filter((item) => !parents.has(item.id) && item.parentId).map((item) => item.id)
})

const dialogTitle = computed(() => (isEdit.value ? 'Alert Template 수정' : '새 Alert Template'))
const selectedGroupPath = computed(() => groupPath(form.groupId))

function groupPath(id) {
  const byId = new Map(groups.value.map((item) => [item.id, item]))
  const parts = []
  let item = byId.get(id)
  while (item) {
    parts.unshift(item.name)
    item = byId.get(item.parentId)
  }
  return parts.join(' / ') || 'Group 미선택'
}

function groupMeta(id) {
  const parts = groupPath(id).split(' / ')
  return parts.length > 1 ? `${parts[0]} / ${parts[1]}` : '2차 Group 선택 후 자동 인식'
}

function resetForm() {
  Object.assign(form, { id: undefined, groupId: leafGroupIds.value[0], name: '', datasourceType: 'prometheus', queryText: '', comparator: '>', threshold: 90, forSeconds: 300, evalIntervalSeconds: 60, severity: 'P2', labelsJson: '{}', annotationsJson: '{}', description: '', status: 1 })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertTemplateList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadSummary() {
  const data = await queryMonitorAlertTemplateList({ pageNum: 1, pageSize: 1 })
  templateTotal.value = data.total || 0
}

async function loadGroups() {
  groups.value = (await queryMonitorAlertTemplateGroups()) || []
}

function selectGroup(id = 0) {
  selectedGroupId.value = id
  query.groupId = id || ''
  query.pageNum = 1
  loadData()
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, keyword: '', datasourceType: '', source: '', groupId: '' })
  selectedGroupId.value = 0
  loadData()
}

function openGroupCreate(parentId = 0) {
  Object.assign(groupForm, { id: undefined, parentId, name: '' })
  groupDialogVisible.value = true
}

function canCreateChild(data) {
  return !data.parentId
}

function openGroupRename(data) {
  Object.assign(groupForm, { id: data.id, parentId: data.parentId, name: data.name })
  groupDialogVisible.value = true
}

async function saveGroup() {
  if (!groupForm.name.trim()) {
    ElMessage.warning('Group 이름을 입력하십시오.')
    return
  }
  await saveMonitorAlertTemplateGroup(groupForm)
  groupDialogVisible.value = false
  await loadGroups()
  ElMessage.success('Template Group을 저장했습니다.')
}

function handleGroupCommand(command, data) {
  if (command === 'child') openGroupCreate(data.id)
  if (command === 'rename') openGroupRename(data)
  if (command === 'delete') removeGroup(data)
}

async function removeGroup(data) {
  await ElMessageBox.confirm(`Group "${data.name}"을(를) 삭제하시겠습니까? 빈 Group만 삭제할 수 있습니다.`, 'Group 삭제', { type: 'warning', confirmButtonText: '삭제', cancelButtonText: '취소' })
  await deleteMonitorAlertTemplateGroup(data.id)
  if (selectedGroupId.value === data.id) selectGroup()
  await loadGroups()
  ElMessage.success('Group을 삭제했습니다.')
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  Object.assign(form, await monitorAlertTemplateInfo(row.id))
  dialogVisible.value = true
}

async function openCopy(row) {
  isEdit.value = false
  const data = await monitorAlertTemplateInfo(row.id)
  Object.assign(form, { ...data, id: undefined, name: `${data.name} - 사용자 정의` })
  dialogVisible.value = true
}

async function openDetail(row) {
  detail.value = await monitorAlertTemplateInfo(row.id)
  detailVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.groupId || !form.queryText.trim()) {
    ElMessage.warning('Template 이름, Template Group, Query 조건을 입력하십시오.')
    return
  }
  saving.value = true
  try {
    await saveMonitorAlertTemplate(form)
    ElMessage.success('Alert Template을 저장했습니다.')
    dialogVisible.value = false
    await Promise.all([loadGroups(), loadSummary(), loadData()])
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`Alert Template "${row.name}"을(를) 삭제하시겠습니까? 생성된 Alert Rule에는 영향을 주지 않습니다.`, 'Template 삭제', { type: 'warning', confirmButtonText: '삭제', cancelButtonText: '취소' })
  await deleteMonitorAlertTemplate(row.id)
  ElMessage.success('Alert Template을 삭제했습니다.')
  await Promise.all([loadGroups(), loadSummary(), loadData()])
}

function useTemplate(row) {
  router.push({ path: '/monitor/alert-rules', query: { templateId: row.id } })
}

function handleTemplateCommand(command, row) {
  if (command === 'detail') openDetail(row)
  if (command === 'copy') openCopy(row)
  if (command === 'edit') openEdit(row)
  if (command === 'delete') remove(row)
}

function datasourceText(type) {
  return ({ prometheus: 'Prometheus', victoriametrics: 'VictoriaMetrics', elasticsearch: 'Elasticsearch', victorialogs: 'VictoriaLogs' }[type] || type || '-')
}

function isPrometheusTemplate(row) {
  return row.datasourceType === 'prometheus' || row.datasourceType === 'victoriametrics'
}

function handleExportSelection(items) {
  exportSelection.value = items.filter(isPrometheusTemplate)
}

function clearExportSelection() {
  templateTableRef.value?.clearSelection()
  exportSelection.value = []
}

async function exportSelectedTemplates() {
  if (!exportSelection.value.length) {
    ElMessage.warning('먼저 Prometheus 또는 VictoriaMetrics Alert Template을 선택하십시오.')
    return
  }
  try {
    await ElMessageBox.confirm(
      `선택한 Metric Template ${exportSelection.value.length}건을 표준 Prometheus Rule YAML로 Export합니다. Export한 내용은 다른 플랫폼의 Template Import 화면에 붙여넣을 수 있습니다.`,
      'Template Export 확인',
      { type: 'info', confirmButtonText: 'Export 확인', cancelButtonText: '취소' }
    )
  } catch {
    return
  }
  exportLoading.value = true
  try {
    const response = await exportPrometheusAlertTemplates(exportSelection.value.map((item) => item.id))
    const blob = new Blob([response.data], { type: response.headers['content-type'] || 'application/x-yaml;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'ops-admin-alert-templates.yaml'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    ElMessage.success(`Template ${exportSelection.value.length}건을 Export했습니다. 바로 다른 플랫폼에 붙여넣어 Import할 수 있습니다.`)
  } finally {
    exportLoading.value = false
  }
}

const prometheusRuleExample = `# 표준 Prometheus Rule YAML: 여러 Rule Group을 동시에 붙여넣을 수 있습니다.
# alert Rule만 Import되며 recording Rule(record)은 무시됩니다.
groups:
  - name: host.rules          # Rule Group 이름, 미리보기용
    interval: 60s             # 평가 간격, 선택 사항, 기본값 60s
    rules:
      - alert: Host CPU 사용률 과다  # Template 이름, 필수
        expr: |
          100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 90
        for: 5m               # 지속 일치 시간, 선택 사항
        labels:
          severity: warning   # critical → P1; warning → P2; info → P3
        annotations:
          summary: Host CPU 사용률 과다
          description: 5분 동안 90%를 초과했습니다. 부하와 리소스 소모가 큰 Process를 확인하십시오.`

function openPrometheusImport() {
  importVisible.value = true
  importStage.value = 'paste'
  importContent.value = ''
  importError.value = ''
  importRows.value = []
  importSelection.value = []
}

async function parsePrometheusContent() {
  if (!importContent.value.trim()) {
    importError.value = 'Prometheus Rule YAML을 붙여넣은 뒤 파싱하십시오.'
    return
  }
  importError.value = ''
  importParsing.value = true
  try {
    importRows.value = (await parsePrometheusAlertTemplates(importContent.value.trim())) || []
    if (!importRows.value.length) {
      importError.value = 'alert Rule을 찾지 못했습니다. groups / rules / alert / expr이 완전한지 확인하십시오.'
      return
    }
    importGroupId.value = leafGroupIds.value.includes(selectedGroupId.value) ? selectedGroupId.value : leafGroupIds.value[0]
    importStage.value = 'preview'
    await nextTick()
    importTableRef.value?.toggleAllSelection()
    ElMessage.success(`Alert Rule ${importRows.value.length}건을 파싱했습니다. 확인한 뒤 Import하십시오.`)
  } catch (error) {
    importError.value = error?.message || 'YAML 파싱에 실패했습니다. 오른쪽 예시에 따라 들여쓰기와 필드를 확인하십시오.'
  } finally {
    importParsing.value = false
  }
}

async function submitPrometheusImport() {
  if (!importGroupId.value) {
    ElMessage.warning('대상 Template Group을 선택하십시오.')
    return
  }
  if (!importSelection.value.length) {
    ElMessage.warning('Alert Rule을 1건 이상 선택하십시오.')
    return
  }
  importSaving.value = true
  try {
    const result = await importPrometheusAlertTemplates({ groupId: importGroupId.value, duplicateStrategy: duplicateStrategy.value, items: importSelection.value })
    ElMessage.success(`Import 완료: 신규 ${result.created || 0}건, 건너뜀 ${result.skipped || 0}건`)
    importVisible.value = false
    await Promise.all([loadGroups(), loadSummary(), loadData()])
  } finally {
    importSaving.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadGroups(), loadSummary(), loadData()])
})
</script>

<template>
  <div class="alert-template-page">
    <section class="template-hero">
      <div class="hero-mark"><el-icon><CollectionTag /></el-icon></div>
      <div class="hero-copy">
        <h2>Alert Template</h2>
        <p>Template Group별로 재사용 가능한 Alert 정의를 축적합니다. Template은 직접 실행되지 않으며 적용 시 Alert Rule이 생성됩니다.</p>
      </div>
      <div class="hero-stats"><span>Template <b>{{ templateTotal }}</b></span><span>Group <b>{{ groups.length }}</b></span></div>
    </section>

    <section class="template-workspace">
      <div class="filter-bar">
        <span class="filter-label">Keyword</span>
        <el-input v-model="query.keyword" clearable placeholder="Template 이름 / Query 조건 / Label" style="width: 248px" @keyup.enter="loadData" />
        <span class="filter-label">Datasource</span>
        <el-select v-model="query.datasourceType" clearable placeholder="전체" style="width: 150px">
          <el-option label="Prometheus" value="prometheus" /><el-option label="VictoriaMetrics" value="victoriametrics" /><el-option label="Elasticsearch" value="elasticsearch" /><el-option label="VictoriaLogs" value="victorialogs" />
        </el-select>
        <span class="filter-label">출처</span>
        <el-select v-model="query.source" clearable placeholder="전체" style="width: 126px"><el-option label="플랫폼" value="platform" /><el-option label="사용자 정의" value="custom" /></el-select>
        <el-button type="primary" @click="loadData"><el-icon><Search /></el-icon>검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
        <div class="filter-actions">
          <el-button @click="openPrometheusImport"><el-icon><DocumentAdd /></el-icon>Prometheus Template 붙여넣기</el-button>
          <el-button :loading="exportLoading" :disabled="!exportSelection.length" @click="exportSelectedTemplates"><el-icon><Download /></el-icon>선택 항목 Export<span v-if="exportSelection.length">({{ exportSelection.length }})</span></el-button>
          <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon>새 Template</el-button>
        </div>
      </div>

      <div class="library-layout">
        <aside class="group-library">
          <div class="tree-heading"><span>컴포넌트 Template Library</span><el-button link type="primary" @click="openGroupCreate()">추가</el-button></div>
          <button class="all-templates" :class="{ active: !selectedGroupId }" @click="selectGroup()"><el-icon><FolderOpened /></el-icon><span>전체 Template</span><b>{{ templateTotal }}</b></button>
          <p class="tree-tip"><el-icon><InfoFilled /></el-icon><span>Group이 분류와 Collector를 결정합니다. 2차 Group을 선택하면 Template 소속이 설정됩니다.</span></p>
          <div class="group-caption">Group 관리</div>
          <el-tree :data="groupTree" node-key="id" :default-expand-all="true" :expand-on-click-node="false" :highlight-current="true" :current-node-key="selectedGroupId" @node-click="(data) => selectGroup(data.id)">
            <template #default="{ data }">
              <span class="group-node">
                <span class="group-name">{{ data.name }}</span><small class="group-count">{{ data.totalCount }}</small>
                <span class="node-actions">
                  <el-button v-if="canCreateChild(data)" link type="primary" title="하위 Group 추가" @click.stop="openGroupCreate(data.id)"><el-icon><Plus /></el-icon></el-button>
                  <el-dropdown trigger="click" @command="(command) => handleGroupCommand(command, data)" @click.stop>
                    <el-button link type="primary" title="추가 Group 작업"><el-icon><MoreFilled /></el-icon></el-button>
                    <template #dropdown><el-dropdown-menu><el-dropdown-item command="rename">이름 변경</el-dropdown-item><el-dropdown-item command="delete" divided class="danger-menu-item">Group 삭제</el-dropdown-item></el-dropdown-menu></template>
                  </el-dropdown>
                </span>
              </span>
            </template>
          </el-tree>
        </aside>

        <main class="template-table-wrap">
          <div v-if="exportSelection.length" class="export-selection-tip"><el-icon><Checked /></el-icon><span>마이그레이션 가능한 Metric Template {{ exportSelection.length }}건을 선택했습니다</span><el-button link type="primary" @click="exportSelectedTemplates">YAML Export</el-button><el-button link @click="clearExportSelection">선택 취소</el-button></div>
          <el-table ref="templateTableRef" v-loading="loading" :data="rows" row-key="id" class="template-table" empty-text="Alert Template이 없습니다. 오른쪽 위에서 Template을 추가해 Rule Library 구축을 시작하십시오." @selection-change="handleExportSelection">
            <el-table-column type="selection" width="48" :selectable="isPrometheusTemplate" reserve-selection />
            <el-table-column label="Template 이름" min-width="270">
              <template #default="{ row }"><button class="template-name" @click="openDetail(row)"><strong>{{ row.name }}</strong><span>{{ row.queryText }}</span></button></template>
            </el-table-column>
            <el-table-column label="Template Group" min-width="205"><template #default="{ row }"><span class="group-path">{{ groupPath(row.groupId) }}</span></template></el-table-column>
            <el-table-column label="Datasource" width="142"><template #default="{ row }">{{ datasourceText(row.datasourceType) }}</template></el-table-column>
            <el-table-column label="Severity" width="86"><template #default="{ row }"><el-tag :type="row.severity === 'P0' || row.severity === 'P1' ? 'danger' : (row.severity === 'P2' ? 'warning' : 'info')" effect="light">{{ row.severity }}</el-tag></template></el-table-column>
            <el-table-column label="출처" width="94"><template #default="{ row }"><el-tag :type="row.source === 'platform' ? 'primary' : 'info'" effect="plain" round>{{ row.source === 'platform' ? '플랫폼' : '사용자 정의' }}</el-tag></template></el-table-column>
            <el-table-column label="작업" width="156" fixed="right">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-button link type="primary" @click="useTemplate(row)">Rule 생성</el-button>
                  <el-dropdown trigger="click" @command="(command) => handleTemplateCommand(command, row)">
                    <el-button link type="primary">더보기<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
                    <template #dropdown><el-dropdown-menu><el-dropdown-item command="detail">상세 보기</el-dropdown-item><el-dropdown-item :command="row.source === 'platform' ? 'copy' : 'edit'">{{ row.source === 'platform' ? 'Template 복제' : 'Template 수정' }}</el-dropdown-item><el-dropdown-item v-if="row.source !== 'platform'" command="delete" divided class="danger-menu-item">Template 삭제</el-dropdown-item></el-dropdown-menu></template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>
        </main>
      </div>
    </section>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="min(920px, 94vw)" top="5vh" destroy-on-close class="template-editor-dialog">
      <el-alert title="Template은 Alert 정의만 저장합니다. Datasource 선택과 Notification 활성화 등 실행 구성은 “Alert Rule”에서 완료하십시오." type="info" :closable="false" show-icon class="dialog-alert" />
      <el-form label-position="top">
        <div class="identity-grid">
          <el-form-item label="Template 이름" required><el-input v-model="form.name" placeholder="예: Host CPU 사용률 과다" /></el-form-item>
          <el-form-item label="Template Group" required><el-cascader v-model="form.groupId" :options="groupTree" :props="{ label: 'name', value: 'id', children: 'children', emitPath: false, checkStrictly: false }" clearable filterable placeholder="2차 Collector Group 선택" style="width: 100%" /></el-form-item>
        </div>
        <p class="group-helper"><el-icon><Connection /></el-icon><span>Group 경로: {{ selectedGroupPath }}. 분류와 Collector는 다음으로 자동 인식됩니다: {{ groupMeta(form.groupId) }}.</span></p>
        <div class="source-row"><el-form-item label="Datasource 유형"><el-select v-model="form.datasourceType" style="width: 260px"><el-option label="Prometheus" value="prometheus" /><el-option label="VictoriaMetrics" value="victoriametrics" /><el-option label="Elasticsearch" value="elasticsearch" /><el-option label="VictoriaLogs" value="victorialogs" /></el-select></el-form-item></div>
        <el-form-item label="Query 조건" required><el-input v-model="form.queryText" type="textarea" :rows="4" placeholder="예: 100 - (avg by (instance) (irate(node_cpu_seconds_total{mode='idle'}[5m])) * 100)" /></el-form-item>
        <div class="threshold-grid">
          <el-form-item label="비교 연산자"><el-select v-model="form.comparator"><el-option v-for="item in ['>', '>=', '<', '<=', '==', '!=']" :key="item" :label="item" :value="item" /></el-select><small>Threshold와 비교합니다</small></el-form-item>
          <el-form-item label="Threshold"><el-input-number v-model="form.threshold" :precision="4" controls-position="right" /><small>단일 Time Series의 현재 값</small></el-form-item>
          <el-form-item label="지속 시간"><el-input-number v-model="form.forSeconds" :min="0" controls-position="right" /><small>연속 일치 후 발생(초)</small></el-form-item>
          <el-form-item label="평가 간격"><el-input-number v-model="form.evalIntervalSeconds" :min="15" controls-position="right" /><small>시스템이 Rule을 실행하는 주기(초)</small></el-form-item>
        </div>
        <el-form-item label="Alert Severity" class="severity-field"><el-radio-group v-model="form.severity"><el-radio-button v-for="item in ['P0', 'P1', 'P2', 'P3']" :key="item" :label="item" /></el-radio-group></el-form-item>
        <div class="json-grid"><el-form-item label="Label JSON"><el-input v-model="form.labelsJson" type="textarea" :rows="2" /></el-form-item><el-form-item label="Annotation JSON"><el-input v-model="form.annotationsJson" type="textarea" :rows="2" /></el-form-item></div>
        <el-form-item label="설명"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="Alert 영향 범위 또는 조치 권고를 입력하십시오" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">취소</el-button><el-button type="primary" :loading="saving" @click="submit">Template 저장</el-button></template>
    </el-dialog>

    <el-dialog v-model="detailVisible" :title="detail.name || 'Alert Template 상세'" width="760px">
      <div class="detail-grid"><div><span>Template Group</span><b>{{ groupPath(detail.groupId) }}</b></div><div><span>Collector</span><b>{{ detail.collector || '-' }}</b></div><div><span>Datasource</span><b>{{ datasourceText(detail.datasourceType) }}</b></div><div><span>트리거 조건</span><b>{{ detail.comparator }} {{ detail.threshold }}, 지속 {{ detail.forSeconds }}초</b></div><div><span>평가 간격</span><b>{{ detail.evalIntervalSeconds }}초</b></div><div><span>Alert Severity</span><b>{{ detail.severity }}</b></div></div>
      <div class="query-preview">{{ detail.queryText }}</div><p class="detail-description">{{ detail.description || '조치 설명이 없습니다.' }}</p>
      <template #footer><el-button @click="detailVisible = false">닫기</el-button><el-button type="primary" @click="detailVisible = false; useTemplate(detail)">Template으로 Rule 생성</el-button></template>
    </el-dialog>

    <el-dialog v-model="groupDialogVisible" :title="groupForm.id ? 'Template Group 이름 변경' : '새 Template Group'" width="440px"><el-form label-position="top"><el-form-item label="Group 이름" required><el-input v-model="groupForm.name" maxlength="32" show-word-limit placeholder="예: node_exporter" /></el-form-item></el-form><template #footer><el-button @click="groupDialogVisible = false">취소</el-button><el-button type="primary" @click="saveGroup">저장</el-button></template></el-dialog>

    <el-dialog v-model="importVisible" title="Template Import" width="min(940px, 94vw)" top="5vh" destroy-on-close class="prometheus-import-dialog">
      <div class="dialog-steps" aria-label="Import 단계"><span :class="{ active: importStage === 'paste' }">1. Rule YAML 붙여넣기</span><i></i><span :class="{ active: importStage === 'preview' }">2. Template 선택</span><i></i><span :class="{ active: importStage === 'preview' }">3. Group Import</span></div>
      <template v-if="importStage === 'paste'">
        <div class="paste-import-layout">
          <section class="yaml-paste-panel">
            <div class="yaml-panel-heading"><div><strong>Prometheus Rule YAML 붙여넣기</strong><p>표준 <code>groups / rules / alert / expr</code> 형식을 지원하며 한 번에 최대 2 MB입니다.</p></div><el-button text type="primary" @click="importContent = prometheusRuleExample">예시 불러오기</el-button></div>
            <el-input v-model="importContent" class="yaml-input" type="textarea" :rows="8" resize="none" spellcheck="false" placeholder="여기에 Prometheus Rule YAML을 붙여넣으십시오" aria-label="Prometheus Rule YAML" @blur="!importContent.trim() && (importError = 'Prometheus Rule YAML을 붙여넣은 뒤 파싱하십시오.')" />
            <p v-if="importError" class="yaml-error" role="alert"><el-icon><WarningFilled /></el-icon>{{ importError }}</p>
          </section>
          <el-collapse class="yaml-example-collapse">
            <el-collapse-item name="example">
              <template #title><div class="example-collapse-title"><el-icon><InfoFilled /></el-icon><span>주석 포함 작성 예시 보기</span><small>주석은 필드 설명용이며 Import되지 않습니다</small></div></template>
              <pre>{{ prometheusRuleExample }}</pre>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>
      <template v-else>
      <div class="import-guide">
        <div class="import-guide-icon"><el-icon><DocumentChecked /></el-icon></div>
        <div><strong>Rule 파일 구조 검증 통과</strong><p><code>alert</code> Rule만 표시하며 recording Rule은 자동으로 무시됩니다. 대상 Group을 확인하고 Import할 Template을 선택하십시오.</p></div>
        <el-tag type="success" effect="light">Import 가능 {{ importRows.length }}건</el-tag>
      </div>
      <div class="import-settings">
        <el-form-item label="대상 Template Group" required>
          <el-cascader v-model="importGroupId" :options="groupTree" :props="{ label: 'name', value: 'id', children: 'children', emitPath: false, checkStrictly: false }" filterable placeholder="2차 Collector Group 선택" />
        </el-form-item>
        <el-form-item label="동일 이름 Template 처리">
          <el-select v-model="duplicateStrategy"><el-option label="동일 이름 Template 건너뛰기(권장)" value="skip" /><el-option label="자동 Rename 후 Import" value="rename" /></el-select>
        </el-form-item>
        <p class="import-target-tip">Import 후 소속: {{ groupPath(importGroupId) }}</p>
      </div>
      <el-table ref="importTableRef" :data="importRows" max-height="430" class="import-preview-table" @selection-change="(items) => importSelection = items">
        <el-table-column type="selection" width="48" />
        <el-table-column label="Template 이름" min-width="220"><template #default="{ row }"><div class="import-name"><strong>{{ row.name }}</strong><span>{{ row.prometheusGroup || '이름 없는 Rule Group' }}</span></div></template></el-table-column>
        <el-table-column label="Query 식" min-width="330"><template #default="{ row }"><code class="import-expression">{{ row.queryText }}</code></template></el-table-column>
        <el-table-column label="트리거 조건" width="130"><template #default="{ row }"><span class="condition-pill">{{ row.comparator }} {{ row.threshold }}</span><small class="condition-duration">지속 {{ row.forSeconds }}s</small></template></el-table-column>
        <el-table-column label="Severity" width="82"><template #default="{ row }"><el-tag :type="row.severity === 'P0' || row.severity === 'P1' ? 'danger' : (row.severity === 'P2' ? 'warning' : 'info')" effect="light">{{ row.severity }}</el-tag></template></el-table-column>
      </el-table>
      <div class="import-footnote"><el-icon><InfoFilled /></el-icon><span><b>{{ importSelection.length }}</b>건을 선택했습니다. Prometheus 식 끝의 수치 비교는 Query 문, 비교 연산자, Threshold로 자동 분리됩니다.</span></div>
      </template>
      <template #footer><el-button @click="importVisible = false">취소</el-button><el-button v-if="importStage === 'preview'" @click="importStage = 'paste'">수정으로 돌아가기</el-button><el-button v-if="importStage === 'paste'" type="primary" :loading="importParsing" @click="parsePrometheusContent">파싱 후 미리보기</el-button><el-button v-else type="primary" :loading="importSaving" :disabled="!importSelection.length" @click="submitPrometheusImport">Template {{ importSelection.length }}건 Import</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.alert-template-page { display: flex; flex-direction: column; gap: 16px; color: #162b4d; }
.template-hero { display: flex; align-items: center; gap: 14px; min-height: 88px; padding: 16px 20px; border: 1px solid #dce8fa; border-radius: 16px; background: linear-gradient(108deg, #fff 4%, #f6f9ff 74%, #eef7ff); box-shadow: 0 10px 24px rgba(43, 74, 128, .06); }
.hero-mark { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 12px; background: #e9f2ff; color: #3477df; font-size: 23px; }.hero-copy h2 { margin: 0 0 5px; font-size: 24px; line-height: 1.2; color: #102747; }.hero-copy p { margin: 0; color: #7184a3; font-size: 13px; }.hero-stats { display: flex; gap: 18px; margin-left: auto; color: #7688a5; font-size: 13px; }.hero-stats b { margin-left: 4px; color: #1c3963; font-size: 18px; }
.template-workspace { overflow: hidden; border: 1px solid #e2eaf6; border-radius: 16px; background: #fff; box-shadow: 0 12px 28px rgba(36, 54, 90, .05); }.filter-bar { display: flex; align-items: center; gap: 10px; min-height: 66px; padding: 10px 16px; border-bottom: 1px solid #edf1f7; background: #fcfdff; }.filter-label { color: #526783; font-size: 13px; font-weight: 700; white-space: nowrap; }.filter-actions { display: flex; align-items: center; gap: 8px; margin-left: auto; }
  .library-layout { display: grid; grid-template-columns: 260px minmax(0, 1fr); gap: 0; }.group-library { min-height: 560px; padding: 16px 12px; border-right: 1px solid #e5ecf7; background: #f9fbfe; }.tree-heading { display: flex; align-items: center; justify-content: space-between; padding: 0 8px 12px; color: #244469; font-size: 14px; font-weight: 700; }.all-templates { display: flex; align-items: center; width: 100%; gap: 8px; min-height: 38px; padding: 8px 10px; border: 0; border-radius: 8px; background: transparent; color: #506785; text-align: left; cursor: pointer; transition: background .18s ease, color .18s ease; }.all-templates:hover { background: #eef4ff; color: #2864cf; }.all-templates.active { background: #e5efff; color: #2769d8; font-weight: 700; }.all-templates span { flex: 1; }.all-templates b { font-size: 12px; }.tree-tip { display: flex; gap: 7px; margin: 12px 0 14px; padding: 10px; border: 1px solid #e4ecfb; border-radius: 8px; background: #f1f6ff; color: #6e82a1; font-size: 12px; line-height: 1.55; }.tree-tip .el-icon { flex: none; margin-top: 2px; color: #4b83e1; }.group-caption { margin: 14px 4px 7px; padding-top: 12px; border-top: 1px solid #e2eaf5; color: #526783; font-size: 12px; font-weight: 700; }.export-selection-tip { display: flex; align-items: center; gap: 8px; min-height: 38px; margin-bottom: 10px; padding: 0 12px; border: 1px solid #d5e5fb; border-radius: 9px; background: #f3f8ff; color: #496a96; font-size: 12px; }.export-selection-tip .el-icon { color: #3678d6; }.export-selection-tip :deep(.el-button:last-child) { margin-left: auto; }
.group-library :deep(.el-tree) { background: transparent; color: #506785; }.group-library :deep(.el-tree-node__content) { height: 34px; border-radius: 7px; }.group-library :deep(.el-tree-node__content:hover), .group-library :deep(.is-current > .el-tree-node__content) { background: #eaf2ff; }.group-node { display: flex; align-items: center; width: 100%; min-width: 0; gap: 6px; padding-right: 4px; }.group-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.group-count { margin-left: auto; color: #8a9ab2; font-size: 11px; font-variant-numeric: tabular-nums; }.node-actions { display: none; align-items: center; margin-left: auto; }.group-node:hover .group-count, .group-library :deep(.is-current .group-count) { display: none; }.group-node:hover .node-actions, .group-library :deep(.is-current .node-actions) { display: flex; }.node-actions :deep(.el-button) { min-width: 26px; min-height: 26px; padding: 0 4px; }.template-table-wrap { min-width: 0; padding: 14px 16px 16px; }.template-table { border: 1px solid #e4ebf5; border-radius: 10px; }.template-table :deep(.el-table__header th) { height: 46px; background: #f5f8fc; color: #526681; font-size: 12px; }.template-table :deep(.el-table__row td) { padding: 11px 0; }.template-name { display: flex; width: 100%; flex-direction: column; gap: 4px; border: 0; background: transparent; text-align: left; cursor: pointer; }.template-name strong { color: #1d365b; font-size: 14px; }.template-name:hover strong { color: #2769d8; }.template-name span { overflow: hidden; color: #8392aa; font: 12px/1.35 Consolas, Monaco, monospace; text-overflow: ellipsis; white-space: nowrap; }.group-path { display: inline-flex; max-width: 100%; padding: 4px 8px; overflow: hidden; border-radius: 6px; background: #f0f5ff; color: #41689f; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.row-actions { display: flex; align-items: center; gap: 4px; white-space: nowrap; }.pager { display: flex; justify-content: flex-end; padding: 14px 2px 0; }
.dialog-alert { margin-bottom: 18px; }.identity-grid, .json-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }.identity-grid :deep(.el-form-item), .json-grid :deep(.el-form-item) { margin-bottom: 12px; }.group-helper { display: flex; gap: 7px; margin: 0 0 14px; padding: 9px 12px; border-radius: 8px; background: #f4f8ff; color: #6680a5; font-size: 12px; line-height: 1.55; }.group-helper .el-icon { flex: none; margin-top: 2px; color: #4b83e1; }.source-row { display: flex; align-items: flex-start; }.source-row :deep(.el-form-item) { margin-bottom: 12px; }.threshold-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin: 4px 0 18px; }.threshold-grid :deep(.el-form-item) { display: flex; flex-direction: column; align-items: stretch; min-height: 150px; margin: 0; padding: 16px; border: 1px solid #f2d49f; border-radius: 14px; background: #fffdf8; box-shadow: 0 6px 16px rgba(169, 118, 35, .045); }.threshold-grid :deep(.el-form-item__label) { width: auto !important; padding: 0 0 10px; color: #425675; font-size: 14px; font-weight: 700; line-height: 20px; }.threshold-grid :deep(.el-form-item__content) { display: flex; width: 100%; flex-direction: column; align-items: stretch; margin: 0 !important; }.threshold-grid :deep(.el-input-number), .threshold-grid :deep(.el-select) { width: 100%; }.threshold-grid small { margin-top: 10px; color: #8393aa; font-size: 12px; line-height: 1.4; }.severity-field { margin-bottom: 14px; }.detail-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; margin-bottom: 16px; }.detail-grid > div { padding: 12px; border: 1px solid #e2eaf6; border-radius: 8px; background: #f9fbfe; }.detail-grid span, .detail-grid b { display: block; }.detail-grid span { margin-bottom: 5px; color: #8090a8; font-size: 12px; }.detail-grid b { color: #234263; font-size: 13px; }.query-preview { padding: 14px; border-radius: 8px; background: #122540; color: #dbeaff; font: 12px/1.65 Consolas, Monaco, monospace; white-space: pre-wrap; word-break: break-all; }.detail-description { color: #687d9b; line-height: 1.65; }
  .import-guide { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; padding: 14px 16px; border: 1px solid #cfe3fa; border-radius: 12px; background: #f4f9ff; }.import-guide-icon { display: grid; width: 38px; height: 38px; flex: none; place-items: center; border-radius: 10px; background: #e1efff; color: #3477df; font-size: 20px; }.import-guide > div:nth-child(2) { flex: 1; }.import-guide strong { color: #1f426d; }.import-guide p { margin: 4px 0 0; color: #7187a6; font-size: 12px; }.import-guide code { color: #356ec8; }.import-settings { display: grid; grid-template-columns: minmax(260px, 1.2fr) minmax(220px, .8fr) 1fr; align-items: end; gap: 14px; margin-bottom: 14px; padding: 14px 16px 2px; border: 1px solid #e2eaf6; border-radius: 10px; background: #fbfcff; }.import-settings :deep(.el-form-item) { margin-bottom: 12px; }.import-settings :deep(.el-cascader), .import-settings :deep(.el-select) { width: 100%; }.import-target-tip { align-self: center; margin: 8px 0 12px; color: #6f83a1; font-size: 12px; }.import-preview-table { border: 1px solid #e1e9f5; border-radius: 10px; }.import-preview-table :deep(th.el-table__cell) { background: #f5f8fc; color: #526681; }.import-name { display: flex; flex-direction: column; gap: 4px; }.import-name strong { color: #1e385e; }.import-name span { color: #8a99af; font-size: 12px; }.import-expression { display: block; overflow: hidden; color: #245fc2; font: 12px/1.5 Consolas, Monaco, monospace; text-overflow: ellipsis; white-space: nowrap; }.condition-pill { display: inline-flex; padding: 3px 7px; border-radius: 6px; background: #edf4ff; color: #3569b7; font-weight: 700; }.condition-duration { display: block; margin-top: 5px; color: #8a98ad; }.import-footnote { display: flex; align-items: center; gap: 7px; margin-top: 12px; color: #6f82a0; font-size: 12px; }.import-footnote .el-icon { color: #4a80db; }.import-footnote b { color: #235ba8; }
  .prometheus-import-dialog :deep(.el-dialog__body) { max-height: calc(100vh - 186px); overflow: auto; }.dialog-steps { display: flex; align-items: center; gap: 10px; margin: -4px 0 18px; color: #98a8bd; font-size: 12px; font-weight: 700; }.dialog-steps span { display: inline-flex; align-items: center; min-height: 28px; padding: 0 10px; border-radius: 999px; background: #f2f5f9; }.dialog-steps span.active { background: #e7f0ff; color: #2c69cf; }.dialog-steps i { width: 24px; height: 1px; background: #dce5ef; }.paste-import-layout { display: flex; flex-direction: column; gap: 10px; }.yaml-paste-panel { min-width: 0; padding: 16px; border: 1px solid #cbdcf5; border-radius: 12px; background: linear-gradient(180deg, #fafdff 0%, #f6f9fe 100%); }.yaml-panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; min-height: 42px; margin-bottom: 12px; }.yaml-panel-heading strong { color: #1d365b; font-size: 14px; }.yaml-panel-heading p { margin: 5px 0 0; color: #7184a2; font-size: 12px; line-height: 1.45; }.yaml-panel-heading code { color: #2867c9; }.yaml-input :deep(textarea) { min-height: 192px !important; padding: 14px; border-color: #cedcf0; background: #fff; color: #273d5e; font: 12px/1.65 Consolas, Monaco, monospace; }.yaml-input :deep(textarea:focus) { border-color: #4b81d8; box-shadow: 0 0 0 3px rgba(69, 123, 214, .12); }.yaml-error { display: flex; align-items: center; gap: 6px; margin: 10px 0 0; color: #dc4d56; font-size: 12px; line-height: 1.45; }.yaml-example-collapse { border: 1px solid #dae5f3; border-radius: 10px; background: #f8fbff; }.yaml-example-collapse :deep(.el-collapse-item__header) { height: 42px; padding: 0 13px; border: 0; border-radius: 10px; background: transparent; }.yaml-example-collapse :deep(.el-collapse-item__wrap) { border: 0; background: #122540; }.yaml-example-collapse :deep(.el-collapse-item__content) { padding: 0; }.example-collapse-title { display: flex; align-items: center; gap: 8px; width: 100%; color: #385879; font-size: 13px; font-weight: 700; }.example-collapse-title .el-icon { color: #3979d8; }.example-collapse-title small { margin-left: auto; color: #8698af; font-size: 12px; font-weight: 400; }.yaml-example-collapse pre { max-height: 250px; margin: 0; padding: 14px 16px; overflow: auto; color: #cce0ff; font: 12px/1.65 Consolas, Monaco, monospace; white-space: pre-wrap; word-break: break-word; }
@media (max-width: 1100px) { .library-layout { grid-template-columns: 1fr; }.group-library { min-height: auto; border-right: 0; border-bottom: 1px solid #e5ecf7; }.filter-bar { align-items: flex-start; flex-wrap: wrap; }.filter-actions { margin-left: 0; } }
  @media (max-width: 800px) { .paste-import-layout { grid-template-columns: 1fr; }.yaml-input :deep(textarea) { min-height: 280px !important; }.yaml-example-panel pre { max-height: 260px; } }
  @media (max-width: 720px) { .hero-stats { display: none; }.identity-grid, .json-grid, .threshold-grid, .detail-grid { grid-template-columns: 1fr; }.dialog-steps i { width: 12px; }.dialog-steps span { padding: 0 7px; font-size: 11px; } }
</style>
