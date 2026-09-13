<script setup>
// I5-J (i5-plan §3.8·CW-4) — AppPipelineCenter.vue 1,076행 분할 자식(실행 폼·
// 실행 상세 다이얼로그). 원본 좌표: 템플릿 :922-977·runDetailVisible :40·
// activeRunStageId :46·logBodyRef :47·runRefreshTimer :48·currentRun :95·
// combinedLog :542-546·activeRunStage :548-551·activeRunLog :553·submitRun
// :419-440·approveCurrentRun :442-448·rollbackCurrentRun :450-455·
// openRunDetail :457-466·startRunRefresh :468-481·stopRunRefresh :483-488·
// selectRunStage :555-560·downloadRunLog :562-571·watch(runDetailVisible)
// :573-575·onBeforeUnmount(stopRunRefresh) :582·CSS .run-summary :1056-1058·
// .run-detail-actions :1059·.run-timeline* :1060-1066·.timeline-* :1063-1069·
// .stage-log-* :1070-1074·.run-log :1075.
// 실행 상세 상태(currentRun·activeRunStageId·runRefreshTimer·logBodyRef·
// runDetailVisible)는 본 자식 전용이라 부모 번들 없이 자식 귀속했다.
// `page` 주입(§5 #11 승계) — reactive 래핑으로 ref/reactive/computed가 언랩된다.
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  approveOpsAppPipelineRun,
  opsAppPipelineRunInfo,
  rollbackOpsAppPipelineRun,
  runOpsAppPipeline
} from '../../../api/ops'
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { apt } from '../../../utils/application-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :40·:46-48·:95 — 실행 상세 전용 상태(자식 귀속)
const runDetailVisible = ref(false)
const activeRunStageId = ref('')
const logBodyRef = ref()
const runRefreshTimer = ref()
const currentRun = ref({ run: {}, stages: [] })

// 원본 :542-546
const combinedLog = computed(() => {
  return (currentRun.value.stages || [])
    .map((stage) => `===== ${stage.stageName} / ${props.page.statusText(stage.status)} =====\n${stage.log || stage.summary || ''}`)
    .join('\n\n')
})

// 원본 :548-551
const activeRunStage = computed(() => {
  const stages = currentRun.value.stages || []
  return stages.find((stage) => String(stage.id) === String(activeRunStageId.value)) || stages[0] || {}
})

// 원본 :553
const activeRunLog = computed(() => activeRunStage.value.log || activeRunStage.value.summary || apt('noStageLog'))

// 원본 :419-440
async function submitRun() {
  let params = {}
  if (props.page.runForm.paramsText.trim()) {
    try {
      params = JSON.parse(props.page.runForm.paramsText)
    } catch {
      ElMessage.warning(apt('paramsJsonRequired'))
      return
    }
  }
  const data = await runOpsAppPipeline({
    pipelineId: props.page.runForm.pipelineId,
    branch: props.page.runForm.branch,
    env: props.page.runForm.env,
    imageTag: props.page.runForm.imageTag,
    params
  })
  ElMessage.success(apt('runStarted', { runId: data.runId }))
  props.page.runVisible = false
  await props.page.loadData()
  await openRunDetail(data.runId)
}

// 원본 :442-448
async function approveCurrentRun(decision) {
  const action = decision === 'approve' ? apt('approveAction') : apt('rejectAction')
  const { value } = await ElMessageBox.prompt(apt('approvalPrompt', { action }), apt('approvalTitle', { action }), { inputType: 'textarea', confirmButtonText: action })
  await approveOpsAppPipelineRun({ runId: currentRun.value.run.id, decision, note: value || '' })
  ElMessage.success(action)
  await openRunDetail(currentRun.value.run.id)
}

// 원본 :450-455
async function rollbackCurrentRun() {
  await ElMessageBox.confirm(apt('rollbackConfirm'), apt('rollbackConfirmTitle'), { type: 'warning' })
  const data = await rollbackOpsAppPipelineRun(currentRun.value.run.id)
  ElMessage.success(apt('rollbackStarted', { runId: data.runId }))
  await openRunDetail(data.runId)
}

// 원본 :457-466 — 실행 상세 로드. 부모 실행 이력 테이블(상세 로그 버튼)과
// 본 자식 제출·승인·롤백에서 진입 — defineExpose 위임.
async function openRunDetail(id) {
  const data = await opsAppPipelineRunInfo(id)
  currentRun.value = { run: data.run || {}, stages: data.stages || [] }
  const preferredStage = currentRun.value.stages.find((stage) => ['failed', 'running', 'waiting_approval'].includes(stage.status)) || currentRun.value.stages[0]
  activeRunStageId.value = preferredStage?.id || ''
  runDetailVisible.value = true
  startRunRefresh()
  await nextTick()
  if (logBodyRef.value) logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
}

// 원본 :468-481
function startRunRefresh() {
  stopRunRefresh()
  if (currentRun.value.run?.status !== 'running') return
  runRefreshTimer.value = window.setInterval(async () => {
    if (!currentRun.value.run?.id) return
    const data = await opsAppPipelineRunInfo(currentRun.value.run.id)
    currentRun.value = { run: data.run || {}, stages: data.stages || [] }
    if (currentRun.value.run.status !== 'running') {
      stopRunRefresh()
      await props.page.loadData()
      if (props.page.activeTab === 'runs') await props.page.loadRuns()
    }
  }, 2500)
}

// 원본 :483-488
function stopRunRefresh() {
  if (runRefreshTimer.value) {
    window.clearInterval(runRefreshTimer.value)
    runRefreshTimer.value = undefined
  }
}

// 원본 :555-560
function selectRunStage(stage) {
  activeRunStageId.value = stage.id
  nextTick(() => {
    if (logBodyRef.value) logBodyRef.value.scrollTop = 0
  })
}

// 원본 :562-571
function downloadRunLog() {
  const name = `${currentRun.value.run.pipelineName || 'pipeline'}-${currentRun.value.run.id || 'run'}.log`.replace(/[\\/:*?"<>|]/g, '_')
  const blob = new Blob([combinedLog.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
}

// 원본 :573-575
watch(runDetailVisible, (visible) => {
  if (!visible) stopRunRefresh()
})

// 원본 :582 — 페이지 언마운트 시 자식도 같이 언마운트되어 동일 시점 해제된다
onBeforeUnmount(stopRunRefresh)

defineExpose({
  openRunDetail
})
</script>

<template>
  <el-dialog v-model="page.runVisible" :title="apt('runPipelineTitle')" width="680px">
    <el-form label-width="100px">
      <el-form-item :label="uiT('pipeline')"><el-input v-model="page.runForm.pipelineName" disabled /></el-form-item>
      <el-form-item :label="apt('runEnvLabel')">
        <el-select v-model="page.runForm.env">
          <el-option label="dev" value="dev" />
          <el-option label="test" value="test" />
          <el-option label="staging" value="staging" />
          <el-option label="prod" value="prod" />
        </el-select>
      </el-form-item>
      <el-form-item label="Branch/Tag"><el-input v-model="page.runForm.branch" /></el-form-item>
      <el-form-item label="Image Version"><el-input :model-value="apt('autoTagPrefix', { tag: (page.runForm.branch || 'main').replace(/[^a-zA-Z0-9_.-]+/g, '-') + '-YYYYMMDDHHmmss' })" disabled /></el-form-item>
      <el-form-item label="Custom Parameter"><el-input v-model="page.runForm.paramsText" type="textarea" :rows="4" :placeholder="apt('paramsPlaceholder')" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="page.runVisible = false">{{ apt('cancel') }}</el-button>
      <el-button type="primary" @click="submitRun">{{ apt('startRun') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="runDetailVisible" :title="apt('runDetailTitle')" width="1080px" class="run-detail-dialog">
    <div class="run-summary">
      <div><span>{{ uiT('pipeline') }}</span><strong>{{ currentRun.run.pipelineName || '-' }}</strong></div>
      <div><span>{{ apt('status') }}</span><el-tag :type="page.statusType(currentRun.run.status)">{{ page.statusText(currentRun.run.status) }}</el-tag></div>
      <div><span>{{ uiT('environment') }}</span><strong>{{ currentRun.run.env || '-' }}</strong></div>
      <div><span>Duration</span><strong>{{ page.durationText(currentRun.run.durationMs) }}</strong></div>
    </div>
    <div class="run-detail-actions">
      <el-button type="primary" link @click="currentRun.run?.id && openRunDetail(currentRun.run.id)">{{ apt('refresh') }}</el-button>
      <el-button type="primary" link @click="downloadRunLog">Log Download</el-button>
      <el-button v-if="currentRun.run.status === 'waiting_approval'" type="success" @click="approveCurrentRun('approve')">{{ apt('approvalApprove') }}</el-button>
      <el-button v-if="currentRun.run.status === 'waiting_approval'" type="danger" @click="approveCurrentRun('reject')">{{ apt('approvalReject') }}</el-button>
      <el-button v-if="currentRun.run.status === 'success'" type="warning" @click="rollbackCurrentRun">{{ apt('rollbackPrevious') }}</el-button>
    </div>
    <div class="run-timeline" aria-label="Pipeline Stage Timeline">
      <button
        v-for="(stage, index) in currentRun.stages"
        :key="stage.id"
        class="run-timeline-stage"
        :class="[stage.status, { active: String(stage.id) === String(activeRunStageId) }]"
        @click="selectRunStage(stage)"
      >
        <span class="timeline-order">{{ index + 1 }}</span>
        <span class="timeline-content"><strong>{{ stage.stageName }}</strong><small>{{ page.stageTypeText(stage.stageType) }} · {{ page.durationText(stage.durationMs) }}</small></span>
        <el-tag size="small" :type="page.statusType(stage.status)">{{ page.statusText(stage.status) }}</el-tag>
      </button>
    </div>
    <div class="stage-log-panel">
      <div class="stage-log-heading">
        <div><span>{{ apt('currentStageLabel') }}</span><strong>{{ activeRunStage.stageName || '-' }}</strong><small>{{ page.stageTypeText(activeRunStage.stageType) }} · {{ page.statusText(activeRunStage.status) }}</small></div>
        <el-button type="primary" link @click="downloadRunLog">{{ apt('fullLogDownload') }}</el-button>
      </div>
      <pre ref="logBodyRef" class="run-log">{{ activeRunLog }}</pre>
    </div>
  </el-dialog>
</template>

<style scoped>
/* 원본 :1056-1058 */
.run-summary { display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: 12px; margin-bottom: 16px; }
.run-summary div { padding: 14px; border: 1px solid #e3ebf7; border-radius: 8px; background: #fbfdff; }
.run-summary span { display: block; margin-bottom: 6px; color: #7d8daa; }

/* 원본 :1059-1066 */
.run-detail-actions { display: flex; justify-content: flex-end; gap: 12px; margin: -4px 0 12px; }
.run-timeline { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 10px; padding: 14px; margin-bottom: 16px; border: 1px solid #e3ebf7; border-radius: 12px; background: #f9fbff; }
.run-timeline-stage { position: relative; display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; gap: 8px; align-items: center; min-height: 70px; padding: 10px; border: 1px solid #e2eaf6; border-radius: 9px; color: #263b5a; background: #fff; text-align: left; cursor: pointer; }
.run-timeline-stage:hover, .run-timeline-stage.active { border-color: #4d7ef3; box-shadow: 0 4px 14px rgba(47, 107, 230, .12); }
.timeline-order { display: grid; width: 26px; height: 26px; place-items: center; border-radius: 50%; color: #5c6f8b; background: #edf2fa; font-weight: 700; }
.run-timeline-stage.success .timeline-order { color: #fff; background: #67c23a; }
.run-timeline-stage.running .timeline-order, .run-timeline-stage.waiting_approval .timeline-order { color: #fff; background: #e6a23c; }
.run-timeline-stage.failed .timeline-order { color: #fff; background: #f56c6c; }

/* 원본 :1067-1069 */
.timeline-content { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.timeline-content strong { overflow: hidden; color: #10213d; text-overflow: ellipsis; white-space: nowrap; }
.timeline-content small { overflow: hidden; color: #8190a8; text-overflow: ellipsis; white-space: nowrap; }

/* 원본 :1070-1075 */
.stage-log-panel { overflow: hidden; border: 1px solid #e3ebf7; border-radius: 12px; }
.stage-log-heading { display: flex; align-items: center; justify-content: space-between; min-height: 70px; padding: 12px 16px; border-bottom: 1px solid #e8eef7; background: #fbfdff; }
.stage-log-heading div { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
.stage-log-heading span, .stage-log-heading small { color: #8392aa; font-size: 12px; }
.stage-log-heading strong { overflow: hidden; color: #203754; text-overflow: ellipsis; white-space: nowrap; }
.run-log { min-height: 480px; max-height: 560px; margin: 0; padding: 16px; overflow: auto; border-radius: 10px; background: #111827; color: #d6e2ff; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.6; white-space: pre-wrap; }
</style>
