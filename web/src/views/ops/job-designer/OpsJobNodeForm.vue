<script setup>
// I5-J (i5-plan §3.8·CW-4) — OpsJobDesigner.vue 1,009행 분할 자식(노드 폼 —
// 선택 단계 설정 패널). 원본 좌표: 템플릿 :798-897(config-panel)·
// selectedScriptTimeout :63-66·selectedScriptVariables :67-70·
// syncSelectedScriptVariables :72-78·handleSelectedScriptChange :80-82·
// updateSelectedHostIds :146-153·updateSelectedGroupIds :155-162·
// CSS .config-panel :955-961 분해·.panel-title :963-967(공유 복사)·
// .form-tip :981-986·.job-variable-* :988-994.
// 선택 상태(selectedNodeId·selectedEdgeId·selectedNodeForm)와 옵션 배열은
// 부모 그래프·로직과 공유해 page로 재배선한다(§5 #11).
import { computed } from 'vue'
import OpsTargetScope from '../components/OpsTargetScope.vue'
import { ot } from '../../../utils/ops-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :63-66
const selectedScriptTimeout = computed(() => {
  const script = props.page.scriptOptions.find((item) => Number(item.id) === Number(props.page.selectedNodeForm.config?.scriptId))
  return script?.timeoutSeconds || 300
})

// 원본 :67-70
const selectedScriptVariables = computed(() => {
  const script = props.page.scriptOptions.find((item) => Number(item.id) === Number(props.page.selectedNodeForm.config?.scriptId))
  return script?.variables || []
})

// 원본 :72-78
function syncSelectedScriptVariables(values = {}) {
  const variables = {}
  selectedScriptVariables.value.forEach((variable) => {
    variables[variable.name] = values[variable.name] ?? (variable.secret ? '' : (variable.defaultValue || ''))
  })
  props.page.selectedNodeForm.config = { ...props.page.selectedNodeForm.config, variables }
}

// 원본 :80-82
function handleSelectedScriptChange() {
  syncSelectedScriptVariables(props.page.selectedNodeForm.config?.variables || {})
}

// 원본 :146-153
function updateSelectedHostIds(hostIds) {
  const normalizedHostIds = props.page.normalizeTargetIds(hostIds)
  props.page.selectedNodeForm.config = {
    ...props.page.selectedNodeForm.config,
    hostIds: normalizedHostIds,
    groupIds: normalizedHostIds.length ? [] : props.page.normalizeTargetIds(props.page.selectedNodeForm.config.groupIds)
  }
}

// 원본 :155-162
function updateSelectedGroupIds(groupIds) {
  const normalizedGroupIds = props.page.normalizeTargetIds(groupIds)
  props.page.selectedNodeForm.config = {
    ...props.page.selectedNodeForm.config,
    hostIds: normalizedGroupIds.length ? [] : props.page.normalizeTargetIds(props.page.selectedNodeForm.config.hostIds),
    groupIds: normalizedGroupIds
  }
}
</script>

<template>
  <div class="page-card config-panel">
    <div class="panel-title">{{ page.selectedEdgeId ? ot('edgeSettings') : ot('stepSettings') }}</div>
    <el-empty v-if="!page.selectedNodeId && !page.selectedEdgeId" :image-size="68" :description="ot('emptySelectionHint')" />
    <el-empty v-else-if="page.selectedEdgeId" :image-size="68" :description="ot('edgeReconnectHint')" />
    <el-form v-else-if="page.selectedNodeId" label-position="top">
      <el-form-item :label="ot('stepName')">
        <el-input v-model="page.selectedNodeForm.label" />
      </el-form-item>

      <template v-if="page.selectedNodeForm.type === 'script'">
        <el-form-item label="Script">
          <el-select v-model="page.selectedNodeForm.config.scriptId" filterable :placeholder="ot('selectScriptShort')" @change="handleSelectedScriptChange">
            <el-option v-for="item in page.scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <div class="job-variable-panel">
          <div class="job-variable-panel__title">{{ ot('stepVariables') }}</div>
          <div class="job-variable-panel__hint">{{ ot('stepVariableHintStart') }}<code>VARIABLE_{{ ot('variableNameWord') }}</code>{{ ot('stepVariableHintEnd') }}</div>
          <div v-if="!selectedScriptVariables.length" class="job-variable-panel__empty">{{ ot('scriptDeclaresNoVariables') }}</div>
          <div v-else class="job-variable-list">
            <div v-for="variable in selectedScriptVariables" :key="variable.name" class="job-variable-field">
              <div class="job-variable-field__label"><code>VARIABLE_{{ variable.name }}</code><el-tag v-if="variable.required" size="small" type="danger" effect="plain">{{ ot('required') }}</el-tag></div>
              <el-input v-model="page.selectedNodeForm.config.variables[variable.name]" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :placeholder="variable.secret ? ot('leaveBlankKeepExisting') : (variable.defaultValue || ot('enterVarValue'))" />
              <div v-if="variable.description" class="job-variable-field__desc">{{ variable.description }}</div>
            </div>
          </div>
        </div>
        <el-form-item label="Concurrency">
          <el-input-number v-model="page.selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
        </el-form-item>
        <el-form-item label="Script Timeout">
          <el-input :model-value="selectedScriptTimeout" disabled>
            <template #append>s</template>
          </el-input>
        </el-form-item>
        <OpsTargetScope
          :host-options="page.hostOptions"
          :group-options="page.groupOptions"
          :host-ids="page.selectedNodeForm.config.hostIds || []"
          :group-ids="page.selectedNodeForm.config.groupIds || []"
          @update:host-ids="updateSelectedHostIds"
          @update:group-ids="updateSelectedGroupIds"
        />
      </template>

      <template v-else-if="page.selectedNodeForm.type === 'file'">
        <el-form-item label="Source Host">
          <el-select v-model="page.selectedNodeForm.config.sourceHostId" filterable :placeholder="ot('selectSourceHost')">
            <el-option v-for="item in page.hostOptions" :key="item.id" :label="item.hostName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source File Path">
          <el-input v-model="page.selectedNodeForm.config.sourcePath" placeholder="/opt/app/config.yml" />
        </el-form-item>
        <el-form-item label="Target Path">
          <el-input v-model="page.selectedNodeForm.config.targetPath" placeholder="/opt/app/config.yml" />
        </el-form-item>
        <el-form-item label="Concurrency">
          <el-input-number v-model="page.selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="ot('timeoutSecondsLabel')">
          <el-input-number v-model="page.selectedNodeForm.config.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="ot('overwriteTargetFile')">
          <el-switch v-model="page.selectedNodeForm.config.overwrite" />
        </el-form-item>
        <OpsTargetScope
          :host-options="page.hostOptions"
          :group-options="page.groupOptions"
          :host-ids="page.selectedNodeForm.config.hostIds || []"
          :group-ids="page.selectedNodeForm.config.groupIds || []"
          @update:host-ids="updateSelectedHostIds"
          @update:group-ids="updateSelectedGroupIds"
        />
      </template>
      <template v-else-if="page.selectedNodeForm.type === 'notify'">
        <el-form-item label="Notification Rule" required>
          <el-select v-model="page.selectedNodeForm.config.notifyRuleId" filterable :placeholder="ot('selectNotificationRulePlaceholder')">
            <el-option v-for="item in page.notifyRuleOptions" :key="item.id" :label="`${item.name} · Job Orchestration`" :value="item.id" />
          </el-select>
          <div class="form-tip">{{ ot('notifyRuleScopeHint') }}</div>
        </el-form-item>
        <el-form-item :label="ot('notifySummary')">
          <el-input v-model="page.selectedNodeForm.config.message" :placeholder="ot('notifySummaryExample')" />
        </el-form-item>
        <el-form-item :label="ot('notifyContent')">
          <el-input v-model="page.selectedNodeForm.config.content" type="textarea" :rows="6" :placeholder="ot('notifyContentPlaceholder')" />
        </el-form-item>
      </template>

      <template v-else>
        <el-form-item :label="ot('confirmMessage')">
          <el-input v-model="page.selectedNodeForm.config.message" :placeholder="ot('confirmMessageExample')" />
        </el-form-item>
        <el-form-item :label="ot('confirmDescription')">
          <el-input v-model="page.selectedNodeForm.config.content" type="textarea" :rows="6" :placeholder="ot('confirmDescriptionPlaceholder')" />
        </el-form-item>
      </template>
    </el-form>
  </div>
</template>

<style scoped>
/* 원본 :955-961 합성 셀렉터 분해 — .config-panel 소속(자식 루트).
    * .left-palette은 부모·.canvas-panel은 OpsJobCanvas 자식 */
.config-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 원본 :963-967 — left-palette(부모)·canvas-panel(OpsJobCanvas)과 공유 복사 */
.panel-title {
  font-size: 18px;
  font-weight: 700;
  color: #14213d;
}

/* 원본 :981-986 */
.form-tip {
  margin-top: 6px;
  color: #8491a9;
  font-size: 12px;
  line-height: 1.6;
}

/* 원본 :988-994 */
.job-variable-panel { padding: 13px; border: 1px solid #d9e6ff; border-radius: 10px; background: linear-gradient(135deg, #f8fbff, #fff); }
.job-variable-panel__title { color: #172744; font-size: 14px; font-weight: 700; }
.job-variable-panel__hint, .job-variable-field__desc { margin-top: 4px; color: #7282a0; font-size: 12px; line-height: 1.55; }
.job-variable-panel code, .job-variable-field__label code { color: #3869d9; }
.job-variable-panel__empty { margin-top: 12px; padding: 9px; color: #8190aa; border: 1px dashed #cbdcff; border-radius: 7px; font-size: 12px; }
.job-variable-list { display: grid; gap: 12px; margin-top: 12px; }
.job-variable-field__label { display: flex; align-items: center; gap: 7px; margin-bottom: 6px; font-size: 12px; font-weight: 600; }
</style>
