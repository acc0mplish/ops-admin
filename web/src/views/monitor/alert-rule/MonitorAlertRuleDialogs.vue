<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorAlertRule.vue 1,037행 분할 자식(프리뷰/매처·템플릿
// 라이브러리·일괄 다이얼로그). 원본 좌표: 템플릿 :859-956·CSS :967-968·:1012-1025·:1035,
// 스크립트 이동분은 각 주석. `page` 주입(§5 #11 승계) — reactive 래핑 언랩 적용.
import { computed, nextTick, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchUpdateMonitorAlertRules, saveMonitorAlertRule } from '../../../api/monitor'
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { mt } from '../../../utils/monitor-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :109-123 — 템플릿 그룹 트리(템플릿 다이얼로그 전용)
const templateGroupTree = computed(() => {
  const map = new Map(props.page.templateGroups.map((item) => [item.id, { ...item, children: [] }]))
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

// 원본 :166-167
const visibleRuleTemplates = computed(() => props.page.managedTemplates)
const selectedManagedTemplates = computed(() => Array.from(props.page.selectedTemplateMap.values()))

// 원본 :102-108 — 일괄 시점 변경 다이얼로그 전용 computed
const batchTimingTitle = computed(() => props.page.batchTimingAction === 'update_for_seconds' ? mt('batchDurationChange') : mt('batchEvalChange'))
const batchTimingLabel = computed(() => props.page.batchTimingAction === 'update_for_seconds' ? 'Duration' : uiT('evaluationInterval'))
const batchTimingTip = computed(() => props.page.batchTimingAction === 'update_for_seconds'
  ? mt('batchDurationTip', { count: props.page.selectedRuleIds.length })
  : mt('batchEvalTip', { count: props.page.selectedRuleIds.length }))
const batchTimingMin = computed(() => props.page.batchTimingAction === 'update_for_seconds' ? 0 : 15)
const batchTimingMax = computed(() => props.page.batchTimingAction === 'update_for_seconds' ? 86400 : 3600)

// 원본 :57 — 템플릿 테이블 ref는 이 자식 템플릿에 귀속
const templateTableRef = ref(null)
// 원본 :58
const restoringTemplateSelection = ref(false)

// 원본 :569-572 — 프리뷰 샘플 라벨(매처) 렌더
function sampleLabel(labels) {
  if (!labels || !Object.keys(labels).length) return '-'
  return Object.entries(labels).map(([key, value]) => `${key}=${value}`).join(', ')
}

// 원본 :343-347
function templateAlertType(template) {
  if (template.datasourceType === 'victorialogs') return 'victorialogs'
  if (template.datasourceType === 'elasticsearch') return 'log'
  return 'metric'
}

// 원본 :349-351
function templateWithType(template) {
  return { ...template, type: templateAlertType(template) }
}

// 원본 :183-192 — 템플릿 그룹 경로(템플릿 다이얼로그 전용)
function templateGroupPath(id) {
  const byId = new Map(props.page.templateGroups.map((item) => [item.id, item]))
  const parts = []
  let item = byId.get(id)
  while (item) {
    parts.unshift(item.name)
    item = byId.get(item.parentId)
  }
  return parts.join(' / ') || mt('uncategorized')
}

// 원본 :353-362
async function restoreTemplateSelection() {
  if (!templateTableRef.value) return
  restoringTemplateSelection.value = true
  templateTableRef.value.clearSelection()
  props.page.managedTemplates.forEach((item) => {
    if (props.page.selectedTemplateMap.has(item.id)) templateTableRef.value.toggleRowSelection(item, true)
  })
  await nextTick()
  restoringTemplateSelection.value = false
}
defineExpose({ restoreTemplateSelection })

// 원본 :364-370
function handleTemplateSelectionChange(selection) {
  if (restoringTemplateSelection.value) return
  const next = new Map(props.page.selectedTemplateMap)
  props.page.managedTemplates.forEach((item) => next.delete(item.id))
  selection.forEach((item) => next.set(item.id, item))
  props.page.selectedTemplateMap = next
}

// 원본 :372-375
function clearTemplateSelection() {
  props.page.selectedTemplateMap = new Map()
  templateTableRef.value?.clearSelection()
}

// 원본 :377-404
function buildTemplateImportPayload(template) {
  const alertType = templateAlertType(template)
  const datasource = props.page.datasourceOptionsForType(alertType)[0]
  if (!datasource) throw new Error(mt('noDatasourceForTemplate', { name: template.name, type: props.page.alertTypeName(alertType) }))
  return {
    name: template.name,
    alertType,
    datasourceScope: 'specific',
    datasourceId: datasource.id,
    query: template.queryText,
    logIndex: template.logIndex || '_all',
    logTimeRangeSeconds: template.logTimeRangeSeconds || 300,
    comparator: template.comparator || '>',
    threshold: Number(template.threshold || 0),
    forSeconds: Number(template.forSeconds || 0),
    evalIntervalSeconds: Number(template.evalIntervalSeconds || 60),
    severity: template.severity || 'P2',
    labelsJson: template.labelsJson || '{}',
    annotationsJson: template.annotationsJson || '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    notifyRepeatIntervalSeconds: 1800,
    maxNotifyCount: 0,
    status: 2,
    description: template.description || ''
  }
}

// 원본 :406-458
async function importSelectedTemplates() {
  const templates = selectedManagedTemplates.value
  if (!templates.length) {
    ElMessage.warning(mt('selectTemplatesFirst'))
    return
  }
  const invalid = templates.filter((item) => !props.page.datasourceOptionsForType(templateAlertType(item))[0])
  if (invalid.length) {
    ElMessage.error(mt('templatesNoDatasource', { names: invalid.map((item) => item.name).join(', ') }))
    return
  }
  try {
    await ElMessageBox.confirm(
      mt('batchCreateConfirm', { count: templates.length }),
      mt('batchCreateTitle'),
      { type: 'warning', confirmButtonText: mt('batchCreateConfirmBtn', { count: templates.length }), cancelButtonText: mt('cancel') }
    )
  } catch {
    return
  }
  props.page.templateImporting = true
  const succeeded = []
  const failed = []
  try {
    for (const template of templates) {
      try {
        await saveMonitorAlertRule(buildTemplateImportPayload(template))
        succeeded.push(template)
      } catch (error) {
        failed.push({ template, message: error?.message || mt('createFailedShort') })
      }
    }
    if (succeeded.length) {
      ElMessage.success(mt('createdDisabledRules', { count: succeeded.length }))
      const remaining = new Map(props.page.selectedTemplateMap)
      succeeded.forEach((item) => remaining.delete(item.id))
      props.page.selectedTemplateMap = remaining
      await props.page.loadData()
    }
    if (failed.length) {
      await ElMessageBox.alert(
        failed.map((item) => `${item.template.name}: ${item.message}`).join('\n'),
        mt('failedToCreateRules', { count: failed.length }),
        { type: 'error', confirmButtonText: mt('backToReview') }
      )
      restoreTemplateSelection()
    } else {
      props.page.templateDialogVisible = false
    }
  } finally {
    props.page.templateImporting = false
  }
}

// 원본 :660-690
async function submitBatchNotify() {
  if (!props.page.batchNotifyRuleId) {
    ElMessage.warning(mt('selectNotifyRule'))
    return
  }
  if (props.page.batchNotifyRepeatIntervalMinutes < 1 || props.page.batchNotifyRepeatIntervalMinutes > 10080) {
    ElMessage.warning(mt('repeatRangeMsg'))
    return
  }
  if (props.page.batchNotifyMaxCount < 0 || props.page.batchNotifyMaxCount > 1000) {
    ElMessage.warning(mt('maxSendRangeMsg'))
    return
  }
  props.page.batchNotifySaving = true
  try {
    await batchUpdateMonitorAlertRules({
      ids: props.page.selectedRuleIds,
      action: 'enable_notify',
      notifyRuleId: props.page.batchNotifyRuleId,
      notifyRepeatIntervalSeconds: props.page.batchNotifyRepeatIntervalMinutes * 60,
      maxNotifyCount: props.page.batchNotifyMaxCount,
      notifyRecoveryEnabled: props.page.batchNotifyRecoveryEnabled
    })
    ElMessage.success(mt('batchNotifyDone'))
    props.page.batchNotifyVisible = false
    props.page.clearSelection()
    await props.page.loadData()
  } finally {
    props.page.batchNotifySaving = false
  }
}

// 원본 :692-712
async function submitBatchTiming() {
  if (props.page.batchTimingValue === undefined || props.page.batchTimingValue === null) {
    ElMessage.warning(mt('enterField', { label: batchTimingLabel.value }))
    return
  }
  props.page.batchTimingSaving = true
  try {
    const isDuration = props.page.batchTimingAction === 'update_for_seconds'
    await batchUpdateMonitorAlertRules({
      ids: props.page.selectedRuleIds,
      action: props.page.batchTimingAction,
      ...(isDuration ? { forSeconds: props.page.batchTimingValue } : { evalIntervalSeconds: props.page.batchTimingValue })
    })
    ElMessage.success(mt('batchChanged', { label: batchTimingLabel.value }))
    props.page.batchTimingVisible = false
    props.page.clearSelection()
    await props.page.loadData()
  } finally {
    props.page.batchTimingSaving = false
  }
}
</script>

<template>
    <el-dialog v-model="page.previewVisible" title="Rule Execution Preview" width="920px" append-to-body>
      <div v-loading="page.previewLoading" class="preview-body">
        <el-alert v-if="page.previewError" :title="page.previewError" type="error" :closable="false" show-icon />
        <el-alert v-else :title="page.previewResult.explanation || mt('previewRunning')" :type="page.previewResult.failedDatasourceCount ? 'warning' : 'success'" :closable="false" show-icon />
        <div class="preview-metrics">
          <div><span>Datasource</span><strong>{{ page.previewResult.datasourceCount || 0 }}</strong></div>
          <div><span>{{ mt('returnedSeries') }}</span><strong>{{ page.previewResult.totalSeries || 0 }}</strong></div>
          <div><span>{{ mt('expectedAlerts') }}</span><strong class="matched">{{ page.previewResult.totalMatched || 0 }}</strong></div>
          <div><span>{{ mt('stateQueryFailed') }}</span><strong>{{ page.previewResult.failedDatasourceCount || 0 }}</strong></div>
        </div>
        <div v-for="item in page.previewResult.results || []" :key="item.datasourceId" class="preview-source">
          <div class="preview-source-head">
            <strong>{{ item.datasourceName }}</strong>
            <el-tag :type="item.status === 'success' ? 'success' : 'danger'">{{ item.status === 'success' ? mt('querySuccess') : mt('stateQueryFailed') }}</el-tag>
          </div>
          <el-alert v-if="item.error" :title="item.error" type="error" :closable="false" />
          <el-table v-else :data="item.samples || []" border max-height="300" :empty-text="mt('noSeriesReturned')">
            <el-table-column :label="mt('triggeredColumn')" width="100"><template #default="{ row }"><el-tag :type="row.matched ? 'danger' : 'success'">{{ row.matched ? mt('conditionMet') : mt('notFired') }}</el-tag></template></el-table-column>
            <el-table-column :label="mt('currentValue')" width="140"><template #default="{ row }">{{ Number(row.value || 0).toFixed(4) }}</template></el-table-column>
            <el-table-column label="Label" min-width="480" show-overflow-tooltip><template #default="{ row }">{{ sampleLabel(row.labels) }}</template></el-table-column>
          </el-table>
        </div>
      </div>
      <template #footer><el-button type="primary" @click="page.previewVisible = false">{{ mt('inactiveShort') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="page.templateDialogVisible" :title="mt('batchCreateFromTemplates')" width="1120px" top="5vh" class="managed-template-dialog" destroy-on-close>
      <div class="template-dialog-head"><div><strong>Alert Template Library</strong><span>{{ mt('templateDialogDesc') }}</span></div><el-input v-model="page.templateKeyword" clearable :placeholder="mt('searchTemplatePlaceholder')" style="width: 300px" @keyup.enter="page.loadManagedTemplates()"><template #prefix><el-icon><Search /></el-icon></template><template #append><el-button :aria-label="mt('searchTemplate')" @click="page.loadManagedTemplates()"><el-icon><Search /></el-icon></el-button></template></el-input></div>
      <div v-loading="page.templateLoading" class="template-library-layout">
        <aside class="template-groups"><button :class="{ active: !page.selectedTemplateGroupId }" @click="page.selectTemplateGroup()"><el-icon><FolderOpened /></el-icon><span>{{ mt('allTemplates') }}</span></button><el-tree :data="templateGroupTree" node-key="id" :default-expand-all="true" :highlight-current="true" :current-node-key="page.selectedTemplateGroupId" @node-click="(data) => page.selectTemplateGroup(data.id)"><template #default="{ data }"><span class="template-group-node"><span>{{ data.name }}</span><small>{{ data.totalCount }}</small></span></template></el-tree></aside>
        <div class="rule-template-list">
          <el-table ref="templateTableRef" :data="visibleRuleTemplates" row-key="id" height="520" class="template-select-table" @selection-change="handleTemplateSelectionChange">
            <el-table-column type="selection" width="48" reserve-selection />
            <el-table-column :label="mt('templateName')" min-width="230">
              <template #default="{ row }"><div class="template-name-cell"><strong>{{ row.name }}</strong><span>{{ row.description || mt('templateFallbackDesc') }}</span></div></template>
            </el-table-column>
            <el-table-column label="Template Group" min-width="170"><template #default="{ row }"><el-tag type="primary" size="small" effect="plain">{{ templateGroupPath(row.groupId) }}</el-tag></template></el-table-column>
            <el-table-column :label="uiT('severity')" width="76"><template #default="{ row }"><el-tag :type="['P0','P1'].includes(row.severity) ? 'danger' : (row.severity === 'P2' ? 'warning' : 'info')" effect="light" size="small">{{ row.severity }}</el-tag></template></el-table-column>
            <el-table-column label="Datasource" width="120"><template #default="{ row }"><span class="template-datasource">{{ row.datasourceType === 'victorialogs' ? 'VictoriaLogs' : (row.datasourceType === 'elasticsearch' ? 'Elasticsearch' : 'Prometheus') }}</span></template></el-table-column>
            <el-table-column :label="mt('searchTemplate')" min-width="260" show-overflow-tooltip><template #default="{ row }"><code class="template-query">{{ row.queryText }}</code></template></el-table-column>
            <el-table-column :label="mt('actions')" width="86" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="page.applyRuleTemplate(templateWithType(row))">{{ mt('configureIndividual') }}</el-button></template></el-table-column>
            <template #empty><el-empty :description="mt('noMatchingTemplates')" /></template>
          </el-table>
        </div>
      </div>
      <template #footer>
        <div class="template-dialog-footer">
          <div class="template-selection-summary"><span>{{ mt('selectedTemplates', { count: selectedManagedTemplates.length }) }}</span><el-button v-if="selectedManagedTemplates.length" link type="primary" @click="clearTemplateSelection">{{ mt('resetSelection') }}</el-button></div>
          <div><el-button @click="page.templateDialogVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="page.templateImporting" :disabled="!selectedManagedTemplates.length" @click="importSelectedTemplates">{{ mt('batchCreateRules') }}<span v-if="selectedManagedTemplates.length"> ({{ selectedManagedTemplates.length }})</span></el-button></div>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="page.batchNotifyVisible" :title="mt('batchEnableNotify')" width="640px" append-to-body>
      <p class="batch-dialog-tip">{{ mt('batchNotifyTip', { count: page.selectedRuleIds.length }) }}</p>
      <el-form label-position="top" class="batch-notify-form">
        <el-form-item label="NotificationRule" required>
          <el-select v-model="page.batchNotifyRuleId" filterable style="width: 100%" :placeholder="mt('selectNotifyRule')">
            <el-option v-for="item in page.notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <div class="batch-notify-grid">
          <el-form-item label="Repeat Notification Interval" required>
            <div class="batch-number-input">
              <el-input-number v-model="page.batchNotifyRepeatIntervalMinutes" :min="1" :max="10080" :step="5" controls-position="right" />
              <span>{{ mt('minuteUnit') }}</span>
            </div>
          </el-form-item>
          <el-form-item label="Maximum Send Count" required>
            <div class="batch-number-input">
              <el-input-number v-model="page.batchNotifyMaxCount" :min="0" :max="1000" controls-position="right" />
              <span>{{ mt('zeroMeansUnlimitedShort') }}</span>
            </div>
          </el-form-item>
        </div>
        <el-form-item class="batch-recovery-field" label="Recovery Notification">
          <div class="batch-recovery-control">
            <el-switch v-model="page.batchNotifyRecoveryEnabled" :active-text="mt('activeShort')" :inactive-text="mt('inactiveShort')" />
            <span>{{ mt('recoverySendDesc') }}</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="page.batchNotifyVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="page.batchNotifySaving" @click="submitBatchNotify">{{ mt('enableConfirmBtn') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="page.batchTimingVisible" :title="batchTimingTitle" width="520px" append-to-body>
      <p class="batch-dialog-tip">{{ batchTimingTip }}</p>
      <el-form label-width="92px">
        <el-form-item :label="batchTimingLabel" required>
          <div class="batch-timing-input">
            <el-input-number v-model="page.batchTimingValue" :min="batchTimingMin" :max="batchTimingMax" :step="page.batchTimingAction === 'update_for_seconds' ? 30 : 15" controls-position="right" />
            <span>{{ mt('secondUnit') }}</span>
          </div>
        </el-form-item>
        <div class="batch-dialog-hint">{{ page.batchTimingAction === 'update_for_seconds' ? mt('timingDurationHint') : mt('timingEvalHint') }}</div>
      </el-form>
      <template #footer><el-button @click="page.batchTimingVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="page.batchTimingSaving" @click="submitBatchTiming">{{ mt('saveChanges') }}</el-button></template>
    </el-dialog>
</template>

<style scoped>
.preview-body { display: flex; flex-direction: column; gap: 14px; min-height: 220px; }.preview-metrics { display: grid; grid-template-columns: repeat(4, 1fr); border: 1px solid #e1e8f3; border-radius: 8px; background: #f7f9fd; }.preview-metrics > div { padding: 13px 16px; border-right: 1px solid #e1e8f3; }.preview-metrics > div:last-child { border-right: 0; }.preview-metrics span, .preview-metrics strong { display: block; }.preview-metrics span { margin-bottom: 4px; color: #8491a8; font-size: 12px; }.preview-metrics strong { color: #20395f; font-size: 20px; }.preview-metrics strong.matched { color: #dc3f48; }.preview-source { padding: 14px; border: 1px solid #e1e8f3; border-radius: 8px; }.preview-source-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; color: #20395f; }
.template-dialog-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 14px; }.template-dialog-head strong, .template-dialog-head span { display: block; }.template-dialog-head strong { color: #1e3155; }.template-dialog-head span { margin-top: 5px; color: #8491a9; font-size: 12px; }.template-library-layout { display: grid; grid-template-columns: 220px minmax(0, 1fr); min-height: 520px; overflow: hidden; border: 1px solid #e1e9f5; border-radius: 12px; background: #fff; }.template-groups { padding: 12px; border-right: 1px solid #e1e9f5; background: #f8faff; }.template-groups > button { display: flex; align-items: center; width: 100%; gap: 8px; min-height: 36px; padding: 8px 10px; border: 0; border-radius: 7px; background: transparent; color: #506785; cursor: pointer; transition: background-color .18s, color .18s; }.template-groups > button:hover { background: #eef4ff; }.template-groups > button.active { background: #e5efff; color: #2769d8; font-weight: 700; }.template-groups > button span { flex: 1; text-align: left; }.template-groups :deep(.el-tree) { margin-top: 8px; background: transparent; }.template-groups :deep(.el-tree-node__content) { height: 34px; border-radius: 7px; }.template-group-node { display: flex; width: 100%; min-width: 0; gap: 6px; padding-right: 6px; }.template-group-node > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.template-group-node small { margin-left: auto; color: #8a99af; }.rule-template-list { min-width: 0; padding: 12px; background: #fff; }.template-select-table { overflow: hidden; border: 1px solid #e4ebf5; border-radius: 10px; }.template-select-table :deep(.el-table__header th) { height: 44px; background: #f5f8fc; color: #526681; font-size: 12px; }.template-select-table :deep(.el-table__row td) { padding: 10px 0; }.template-select-table :deep(.el-table__row:hover > td) { background: #f7faff; }.template-name-cell { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.template-name-cell strong { overflow: hidden; color: #1b3559; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }.template-name-cell span { overflow: hidden; color: #8492a8; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.template-datasource { color: #536b8b; font-size: 12px; }.template-query { display: block; overflow: hidden; color: #4264c2; font: 12px/1.45 Consolas, Monaco, monospace; text-overflow: ellipsis; white-space: nowrap; }.template-dialog-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; gap: 16px; }.template-dialog-footer > div:last-child { display: flex; gap: 8px; }.template-selection-summary { display: flex; align-items: center; gap: 10px; color: #71819a; font-size: 13px; }.template-selection-summary b { color: #2f67d8; font-size: 16px; }.batch-dialog-tip { margin: 0 0 18px; color: #7282a0; line-height: 1.65; }
.batch-timing-input { display: flex; align-items: center; gap: 8px; }
.batch-timing-input :deep(.el-input-number) { width: 220px; }
.batch-timing-input span { color: #60718c; font-size: 13px; }
.batch-dialog-hint { margin: -6px 0 0 92px; color: #8a98ad; font-size: 12px; line-height: 1.55; }
.batch-notify-form :deep(.el-form-item) { margin-bottom: 16px; }
.batch-notify-form :deep(.el-form-item__label) { color: #465a78; font-weight: 600; }
.batch-notify-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.batch-number-input { display: flex; align-items: center; gap: 8px; }
.batch-number-input :deep(.el-input-number) { width: 170px; }
.batch-number-input span { color: #72829a; font-size: 12px; white-space: nowrap; }
.batch-recovery-field { margin-bottom: 0 !important; padding: 13px 14px; border: 1px solid #dce7f7; border-radius: 9px; background: #f8fbff; }
.batch-recovery-field :deep(.el-form-item__label) { margin-bottom: 8px; }
.batch-recovery-control { display: flex; align-items: center; gap: 12px; }
.batch-recovery-control span { color: #7587a2; font-size: 12px; }
@media (max-width: 1000px) {
  .template-library-layout { grid-template-columns: 1fr; }
  .template-groups { max-height: 190px; overflow: auto; border-right: 0; border-bottom: 1px solid #e1e9f5; }
  .template-dialog-head { align-items: flex-start; flex-direction: column; }
  .template-dialog-head :deep(.el-input) { width: 100% !important; }
}
@media (max-width: 700px) {
  .preview-metrics { grid-template-columns: 1fr; }
  .template-dialog-footer { align-items: flex-end; flex-direction: column; gap: 10px; }
  .template-selection-summary { align-self: flex-start; }
  .batch-notify-grid { grid-template-columns: 1fr; gap: 0; }
}
</style>
