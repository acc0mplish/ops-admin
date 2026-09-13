<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorAlertRule.vue 1,037행 분할 자식(룰 폼 다이얼로그).
// 원본 좌표: 템플릿 :790-857·CSS :966·:971-1011·:1029-1036·스크립트 이동분은 각 주석.
// `page` 주입(§5 #11 승계) — reactive 래핑으로 ref/computed가 언랩된다.
import { computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { previewMonitorAlertRule, saveMonitorAlertRule } from '../../../api/monitor'
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { mt } from '../../../utils/monitor-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :164 — 다이얼로그 전용 computed
const dialogTitle = computed(() => (props.page.copyMode ? mt('duplicateAlertRule') : (props.page.isEdit ? mt('editAlertRule') : mt('addAlertRule'))))
// 원본 :98-138 — 에디터 폼 전용 computed(자식 귀속)
const alertTypeLabel = computed(() => props.page.alertTypeName(props.page.form.alertType))
const isLogAlert = computed(() => ['log', 'victorialogs'].includes(props.page.form.alertType))
const queryLabel = computed(() => {
  if (props.page.form.alertType === 'log') return 'Elasticsearch Query'
  if (props.page.form.alertType === 'victorialogs') return 'LogsQL Query'
  return 'PromQL'
})
const queryPlaceholder = computed(() => {
  if (props.page.form.alertType === 'log') return mt('logQueryPlaceholderEs')
  if (props.page.form.alertType === 'victorialogs') return mt('logQueryPlaceholderVl')
  return mt('logQueryPlaceholderProm')
})
const queryHint = computed(() => {
  if (props.page.form.alertType === 'log') return mt('queryHintEs')
  if (props.page.form.alertType === 'victorialogs') return mt('queryHintVl')
  return mt('queryHintProm')
})

// 원본 :194-198
function defaultQueryForType(type) {
  if (type === 'log') return 'level:ERROR OR level:FATAL'
  if (type === 'victorialogs') return '_msg:~"(?i)(error|fatal)"'
  return ''
}

// 원본 :229-234 — 폼 타입 전환(초안 쿼리 보존 순서 원문 유지)
function handleAlertTypeChange(type) {
  props.page.queryDrafts[props.page.previousAlertType] = props.page.form.queryText
  props.page.form.queryText = props.page.queryDrafts[type] || defaultQueryForType(type)
  props.page.previousAlertType = type
  props.page.form.datasourceId = props.page.form.datasourceScope === 'specific' ? props.page.datasourceOptionsForType(type)[0]?.id : undefined
}

// 원본 :236-238
function applyDatasourceScope() {
  props.page.form.datasourceId = props.page.form.datasourceScope === 'specific' ? props.page.availableDatasourceOptions[0]?.id : undefined
}

// 원본 :543-549
function buildRulePayload() {
  return {
    ...props.page.form,
    query: props.page.form.queryText,
    notifyRepeatIntervalSeconds: props.page.form.notifyRepeatIntervalMinutes * 60
  }
}

// 원본 :551-567 — Rule Test(프리뷰) 트리거. 프리뷰 상태는 page로 공유된다.
async function handlePreview() {
  if (!props.page.form.queryText.trim() || (props.page.form.datasourceScope === 'specific' && !props.page.form.datasourceId)) {
    ElMessage.warning(mt('enterQueryFirst', { datasource: props.page.form.datasourceScope === 'specific' ? mt('dsAnd') : '', query: props.page.form.alertType === 'log' ? mt('esQueryLabel') : mt('promqlLabel') }))
    return
  }
  props.page.previewVisible = true
  props.page.previewLoading = true
  props.page.previewError = ''
  props.page.previewResult = { results: [] }
  try {
    props.page.previewResult = await previewMonitorAlertRule(buildRulePayload())
  } catch (error) {
    props.page.previewError = error?.message || mt('previewFailedMsg')
  } finally {
    props.page.previewLoading = false
  }
}

// 원본 :503-513
function validateJsonFields() {
  for (const [label, value] of [['Label JSON', props.page.form.labelsJson], ['Annotation JSON', props.page.form.annotationsJson]]) {
    try {
      JSON.parse(value || '{}')
    } catch {
      ElMessage.warning(mt('invalidJsonFormat', { label }))
      return false
    }
  }
  return true
}

// 원본 :515-541
async function submit() {
  if (!props.page.form.name.trim() || !props.page.form.queryText.trim() || (props.page.form.datasourceScope === 'specific' && !props.page.form.datasourceId)) {
    ElMessage.warning(mt('enterRuleRequired', { datasource: props.page.form.datasourceScope === 'specific' ? mt('dsAnd') : '', query: props.page.form.alertType === 'log' ? mt('esQueryLabel') : mt('promqlLabel') }))
    return
  }
  if (props.page.form.notifyEnabled && !props.page.form.notifyRuleId) {
    ElMessage.warning(mt('selectNotifyRule'))
    return
  }
  if (!validateJsonFields()) return
  if (props.page.form.status === 1 && props.page.form.datasourceScope === 'all') {
    await ElMessageBox.confirm(mt('globalRuleConfirm'), mt('globalRuleTitle'), { type: 'warning', confirmButtonText: mt('enableConfirmBtn'), cancelButtonText: mt('backToReview') })
  }
  props.page.saving = true
  try {
    await saveMonitorAlertRule({
      ...props.page.form,
      query: props.page.form.queryText,
      notifyRepeatIntervalSeconds: props.page.form.notifyRepeatIntervalMinutes * 60
    })
    ElMessage.success(mt('savedMsg'))
    props.page.dialogVisible = false
    await props.page.loadData()
  } finally {
    props.page.saving = false
  }
}
</script>

<template>
    <el-dialog v-model="page.dialogVisible" :title="dialogTitle" width="min(1040px, 94vw)" top="4vh" class="rule-editor-dialog" destroy-on-close>
      <div class="editor-intro"><div><el-icon><Operation /></el-icon></div><span><strong>{{ mt('executionRuleSetup') }}</strong><small>{{ mt('editorIntroDesc') }}</small></span><el-button plain @click="page.openTemplateDialog">{{ mt('selectAlertTemplate') }}</el-button></div>
      <el-form label-position="top" class="rule-editor-form">
        <section class="form-section"><div class="section-heading"><span>01</span><div><strong>{{ mt('basicInfo') }}</strong><small>{{ mt('basicInfoDesc') }}</small></div></div>
        <div class="form-grid"><el-form-item :label="mt('ruleName')" required><el-input v-model="page.form.name" :placeholder="mt('ruleNamePlaceholder')" /></el-form-item><el-form-item :label="mt('ruleStatus')"><el-radio-group v-model="page.form.status"><el-radio-button :label="2">{{ mt('draftRadio') }}</el-radio-button><el-radio-button :label="1">{{ mt('enabledOption') }}</el-radio-button></el-radio-group></el-form-item></div>
        <el-form-item label="Alert Type" required>
          <el-radio-group v-model="page.form.alertType" @change="handleAlertTypeChange">
            <el-radio-button label="metric">Metric Alert / PromQL</el-radio-button>
            <el-radio-button label="log">Log Alert / Elasticsearch</el-radio-button>
            <el-radio-button label="victorialogs">Log Alert / VictoriaLogs</el-radio-button>
          </el-radio-group>
        </el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>02</span><div><strong>{{ mt('dsAndQuery') }}</strong><small>{{ mt('dsQueryDesc') }}</small></div></div>
        <el-form-item label="Datasource Scope" required>
          <div class="datasource-scope">
            <el-radio-group v-model="page.form.datasourceScope" @change="applyDatasourceScope">
              <el-radio-button label="all">{{ mt('matchAllDatasources', { type: alertTypeLabel }) }}</el-radio-button>
              <el-radio-button label="specific">{{ mt('specificDatasource') }}</el-radio-button>
            </el-radio-group>
            <el-select v-if="page.form.datasourceScope === 'specific'" v-model="page.form.datasourceId" filterable style="width: 360px" :placeholder="mt('selectDatasource')">
              <el-option v-for="item in page.availableDatasourceOptions" :key="item.id" :label="`${item.name} (${item.type})`" :value="item.id" />
            </el-select>
            <span v-else class="form-tip">{{ mt('runOnAllDatasources', { type: page.form.alertType === 'log' ? 'Elasticsearch' : 'Prometheus / VictoriaMetrics' }) }}</span>
          </div>
        </el-form-item>
        <el-form-item v-if="page.form.alertType === 'log'" label="Log Index" required><el-input v-model="page.form.logIndex" :placeholder="mt('logIndexPlaceholder')" /></el-form-item>
        <el-form-item :label="queryLabel" required>
          <el-input v-model="page.form.queryText" type="textarea" :rows="4" :placeholder="queryPlaceholder" />
          <div class="form-tip">{{ queryHint }}</div>
        </el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>03</span><div><strong>Trigger Condition</strong><small>{{ mt('triggerConditionDesc') }}</small></div></div>
        <section class="rule-parameters">
          <div class="parameter-card">
            <span>Comparator</span>
            <el-select v-model="page.form.comparator"><el-option v-for="item in ['>','>=','<','<=','==','!=']" :key="item" :label="item" :value="item" /></el-select>
            <small>{{ mt('compareThreshold') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ isLogAlert ? 'Match Threshold' : 'Threshold' }}</span>
            <el-input-number v-model="page.form.threshold" :precision="4" :step="isLogAlert ? 1 : 0.1" controls-position="right" />
            <small>{{ isLogAlert ? mt('windowMatchCount') : mt('seriesCurrentValue') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ isLogAlert ? 'Query Window' : 'Duration' }}</span>
            <div class="parameter-number"><el-input-number v-if="isLogAlert" v-model="page.form.logTimeRangeSeconds" :min="60" :max="86400" controls-position="right" /><el-input-number v-else v-model="page.form.forSeconds" :min="0" :max="86400" controls-position="right" /><b>{{ mt('secondUnit') }}</b></div>
            <small>{{ isLogAlert ? mt('recentLogAgg') : mt('consecutiveMatchTrigger') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ uiT('evaluationInterval') }}</span>
            <div class="parameter-number"><el-input-number v-model="page.form.evalIntervalSeconds" :min="15" :max="3600" controls-position="right" /><b>{{ mt('secondUnit') }}</b></div>
            <small>{{ mt('systemRuleInterval') }}</small>
          </div>
        </section>
        <el-form-item label="Alert Severity"><el-radio-group v-model="page.form.severity"><el-radio-button v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" /></el-radio-group></el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>04</span><div><strong>{{ mt('notifyAndContext') }}</strong><small>{{ mt('notifyContextDesc') }}</small></div></div>
        <div class="json-grid"><el-form-item label="Label JSON"><el-input v-model="page.form.labelsJson" type="textarea" :rows="2" :placeholder="mt('labelJsonPlaceholder')" /></el-form-item><el-form-item label="Annotation JSON"><el-input v-model="page.form.annotationsJson" type="textarea" :rows="2" :placeholder="mt('annotationJsonPlaceholder')" /></el-form-item></div>
        <div class="notify-toggle" :class="{ 'is-enabled': page.form.notifyEnabled }">
          <div class="notify-toggle-copy"><div class="notify-toggle-icon"><el-icon><Bell /></el-icon></div><span><strong>Notification</strong><small>{{ page.form.notifyEnabled ? mt('notifyOnDesc') : mt('notifyOffDesc') }}</small></span></div>
          <div class="notify-toggle-action"><el-tag :type="page.form.notifyEnabled ? 'success' : 'info'" effect="light">{{ page.form.notifyEnabled ? mt('activeShort') : mt('inactiveShort') }}</el-tag><el-switch v-model="page.form.notifyEnabled" /></div>
        </div>
        <div v-if="page.form.notifyEnabled" class="notify-settings"><el-form-item label="NotificationRule" required><el-select v-model="page.form.notifyRuleId" filterable style="width: 100%"><el-option v-for="item in page.notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item><div class="form-grid"><el-form-item label="Repeat Notification Interval"><div class="parameter-number"><el-input-number v-model="page.form.notifyRepeatIntervalMinutes" :min="1" :max="10080" controls-position="right" /><b>{{ mt('minuteUnit') }}</b></div></el-form-item><el-form-item label="Maximum Send Count"><el-input-number v-model="page.form.maxNotifyCount" :min="0" :max="1000" controls-position="right" /><div class="form-tip">{{ mt('zeroMeansUnlimited') }}</div></el-form-item></div><el-form-item label="Recovery Notification"><el-switch v-model="page.form.notifyRecoveryEnabled" /></el-form-item></div>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="page.form.description" type="textarea" :rows="2" :placeholder="mt('descriptionPlaceholder')" /></el-form-item></section>
      </el-form>
      <template #footer><div class="rule-editor-footer"><span>{{ mt('testFirstHint') }}</span><div><el-button @click="page.dialogVisible = false">{{ mt('cancel') }}</el-button><el-button :loading="page.previewLoading" @click="handlePreview">Rule Test</el-button><el-button type="primary" :loading="page.saving" @click="submit">{{ mt('saveRule') }}</el-button></div></div></template>
    </el-dialog>
</template>

<style scoped>
:global(.rule-editor-dialog .el-dialog__body) { max-height: calc(100vh - 188px); overflow: auto; padding-top: 14px; }:global(.rule-editor-dialog .el-dialog__footer) { margin-top: 0; padding-top: 12px; border-top: 1px solid #edf1f7; }.editor-intro { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; padding: 13px 14px; border: 1px solid #dce8fa; border-radius: 10px; background: #f3f7ff; }.editor-intro > div { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 9px; background: #e2edff; color: #3872d6; }.editor-intro > span { flex: 1; }.editor-intro strong, .editor-intro small { display: block; }.editor-intro strong { color: #234369; }.editor-intro small { margin-top: 3px; color: #7588a6; }.rule-editor-form { display: flex; flex-direction: column; gap: 14px; }.form-section { padding: 16px; border: 1px solid #e0e8f4; border-radius: 12px; background: #fff; }.section-heading { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }.section-heading > span { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 8px; background: #e9f1ff; color: #3470d7; font-size: 12px; font-weight: 700; }.section-heading strong, .section-heading small { display: block; }.section-heading strong { color: #1e385e; }.section-heading small { margin-top: 2px; color: #8492a8; font-size: 12px; }.form-grid, .json-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }.form-section :deep(.el-form-item:last-child) { margin-bottom: 0; }.datasource-scope { display: flex; align-items: center; gap: 12px; width: 100%; flex-wrap: wrap; }.form-tip { margin-top: 6px; color: #8491a9; font-size: 12px; line-height: 1.55; }.datasource-scope .form-tip { margin: 0; }.rule-parameters { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-bottom: 16px; }.parameter-card { min-height: 126px; padding: 14px; border: 1px solid #dfe7f2; border-radius: 10px; background: #f8faff; display: flex; flex-direction: column; gap: 9px; }.parameter-card > span { color: #51617b; font-size: 13px; font-weight: 700; }.parameter-card :deep(.el-input-number), .parameter-card :deep(.el-select) { width: 100%; }.parameter-card small { color: #8c98ad; font-size: 12px; line-height: 1.4; }.parameter-number { display: flex; align-items: center; gap: 6px; }.parameter-number :deep(.el-input-number) { flex: 1; min-width: 0; }.parameter-number b { color: #71809b; font-size: 12px; font-weight: 500; }.notify-toggle { display: flex; align-items: center; justify-content: space-between; min-height: 58px; padding: 10px 14px; border: 1px solid #e1e9f5; border-radius: 9px; background: #f8faff; }.notify-toggle strong, .notify-toggle small { display: block; }.notify-toggle small { margin-top: 3px; color: #8492a8; }.notify-settings { margin-top: 12px; padding: 14px; border: 1px solid #dbe7f8; border-radius: 10px; background: #f8fbff; }.rule-editor-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; }.rule-editor-footer > span { color: #8190a5; font-size: 12px; }
/* Rule editor refinements: keep the trigger controls compact and scannable. */
.rule-editor-dialog .rule-parameters {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin: 4px 0 18px;
}
.rule-editor-dialog .parameter-card {
  min-height: 150px;
  padding: 16px;
  gap: 12px;
  border-color: #f1d39c;
  border-radius: 14px;
  background: #fffdf8;
  box-shadow: 0 8px 22px rgba(181, 129, 39, .06);
}
.rule-editor-dialog .parameter-card > span { color: #425675; font-size: 14px; }
.rule-editor-dialog .parameter-card small { margin-top: auto; color: #7184a3; }
.rule-editor-dialog .notify-toggle {
  gap: 16px;
  min-height: 74px;
  margin: 2px 0 14px;
  padding: 14px 16px;
  border-color: #cfdced;
  border-radius: 12px;
  background: #f7faff;
  transition: border-color .18s ease, background-color .18s ease, box-shadow .18s ease;
}
.rule-editor-dialog .notify-toggle.is-enabled {
  border-color: #96c1f5;
  background: linear-gradient(100deg, #eff7ff 0%, #f8fbff 100%);
  box-shadow: 0 8px 20px rgba(58, 124, 207, .10);
}
.notify-toggle-copy, .notify-toggle-action { display: flex; align-items: center; }
.notify-toggle-copy { min-width: 0; gap: 12px; }
.notify-toggle-copy > span { min-width: 0; }
.notify-toggle-icon { display: grid; flex: none; width: 38px; height: 38px; place-items: center; border-radius: 10px; background: #e8f1ff; color: #3477df; font-size: 18px; }
.notify-toggle.is-enabled .notify-toggle-icon { background: #dff1e7; color: #2c9b62; }
.notify-toggle strong { color: #244267; font-size: 14px; }
.notify-toggle small { line-height: 1.5; }
.notify-toggle-action { flex: none; gap: 12px; }
.rule-editor-dialog .notify-settings { margin: -2px 0 18px; padding: 16px; border-color: #bcd7f6; border-radius: 12px; background: #f7fbff; box-shadow: inset 3px 0 0 #6aa8f8; }
@media (max-width: 980px) {
  .rule-editor-dialog .rule-parameters { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 700px) {
  .rule-editor-dialog .rule-parameters { grid-template-columns: 1fr; }
  .rule-editor-dialog .notify-toggle { align-items: flex-start; flex-direction: column; }
  .notify-toggle-action { width: 100%; justify-content: space-between; }
}
</style>
