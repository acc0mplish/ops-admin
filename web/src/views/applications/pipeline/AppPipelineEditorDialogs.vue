<script setup>
// I5-J (i5-plan §3.8·CW-4) — AppPipelineCenter.vue 1,076행 분할 자식(템플릿
// 선택·파이프라인 편집 다이얼로그). 원본 좌표: 템플릿 :741-920·categories :50·
// filteredTemplates :98-102·categoryLabel :104-106·resetForm :108-122·
// stringifyDefinition :144-146·defaultStage :148-171·stageHint :173-185·
// stageTone :187-189·normalizeStageConfig :191-217·dockerBuildStages :219-221·
// registryName :223-226·createBlankPipeline :293-298·useTemplate :300-302·
// confirmTemplate :304-317·fillFromApp :338-342·addStage :344-346·
// removeStage :348-350·moveStage :352-357·submitPipeline :374-402·
// CSS .template-picker :1000-1004·.template-grid/.template-card/.blank-template
// :1005-1011(탭과 공유 복사)·.form-grid :1012·.stage-* :1013-1033·
// .image-stage-* :1034-1054·미디어 900 :1055.
// `page` 주입(§5 #11 승계) — reactive 래핑으로 ref/reactive/computed가 언랩된다.
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { saveOpsAppPipeline } from '../../../api/ops'
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { apt } from '../../../utils/application-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :50 — 템플릿 선택 다이얼로그 전용 상수(자식 귀속)
const categories = ['all', 'Java', 'Node.js', 'Go', 'Python', 'Vue', 'blank']

// 원본 :98-102 — 편집 다이얼로그 전용 computed(자식 귀속)
const filteredTemplates = computed(() => {
  if (props.page.selectedCategory === 'all') return props.page.templates
  if (props.page.selectedCategory === 'blank') return []
  return props.page.templates.filter((item) => item.category === props.page.selectedCategory || item.techStack === props.page.selectedCategory)
})

// 원본 :104-106
function categoryLabel(category) {
  return { all: apt('categoryAll'), blank: apt('categoryBlank') }[category] || category
}

// 원본 :108-122
function resetForm() {
  Object.assign(props.page.form, {
    id: undefined,
    name: '',
    appId: undefined,
    defaultBranch: '',
    env: 'test',
    techStack: 'custom',
    templateId: 0,
    executorHostId: undefined,
    status: 1,
    description: '',
    stages: []
  })
}

// 원본 :144-146
function stringifyDefinition() {
  return JSON.stringify({ stages: props.page.form.stages }, null, 2)
}

// 원본 :148-171
function defaultStage(type = 'command') {
  const next = props.page.form.stages.length + 1
  const names = {
    checkout: 'Source Checkout',
    command: 'Build Command',
    test: 'Unit Test',
    dockerBuild: 'Docker Image Build',
    dockerPush: 'Image Registry Push',
    k8sDeploy: 'Kubernetes Deploy',
    manual: 'Manual Approval',
    notify: 'Notification'
  }
  const stage = {
    id: `${type}-${Date.now()}-${next}`,
    name: names[type] || `Stage ${next}`,
    type,
    timeoutSeconds: 1800,
    failurePolicy: type === 'notify' ? 'ignore' : 'stop',
    config: {},
    env: {}
  }
  normalizeStageConfig(stage)
  return stage
}

// 원본 :173-185
function stageHint(type) {
  return {
    checkout: apt('hintCheckout'),
    command: apt('hintCommand'),
    test: apt('hintTest'),
    build: apt('hintBuild'),
    dockerBuild: apt('hintDockerBuild'),
    dockerPush: apt('hintDockerPush'),
    k8sDeploy: apt('hintK8sDeploy'),
    manual: apt('hintManual'),
    notify: apt('hintNotify')
  }[type] || apt('hintDefault')
}

// 원본 :187-189
function stageTone(type) {
  return { checkout: 'source', command: 'command', test: 'test', build: 'build', dockerBuild: 'image', dockerPush: 'push', k8sDeploy: 'deploy', manual: 'manual', notify: 'notify' }[type] || 'command'
}

// 원본 :191-217
function normalizeStageConfig(stage) {
  if (!stage.config || typeof stage.config !== 'object') stage.config = {}
  if (['command', 'test', 'build'].includes(stage.type) && stage.config.script === undefined) {
    stage.config.script = ''
  }
  if (stage.type === 'dockerBuild') {
    if (!stage.config.registryId) stage.config.registryId = undefined
    if (!stage.config.dockerfile) stage.config.dockerfile = 'Dockerfile'
    if (!stage.config.context) stage.config.context = '.'
  }
  if (stage.type === 'dockerPush') {
    if (!stage.config.sourceStageId) stage.config.sourceStageId = undefined
    if (!stage.config.loginMode) stage.config.loginMode = 'registry'
  }
  if (stage.type === 'k8sDeploy') {
    if (!stage.config.clusterId) stage.config.clusterId = undefined
    if (!stage.config.workloadType) stage.config.workloadType = 'deployment'
    if (!stage.config.namespace) stage.config.namespace = ''
    if (!stage.config.workload) stage.config.workload = ''
    if (!stage.config.container) stage.config.container = ''
    if (!stage.config.repository) stage.config.repository = ''
    if (!stage.config.healthUrl) stage.config.healthUrl = ''
  }
  if (stage.type === 'notify' && !stage.config.notifyRuleId) {
    stage.config.notifyRuleId = undefined
  }
}

// 원본 :219-221
function dockerBuildStages(excludeId) {
  return props.page.form.stages.filter((item) => item.type === 'dockerBuild' && item.id !== excludeId)
}

// 원본 :223-226
function registryName(id) {
  const registry = props.page.imageRegistryOptions.find((item) => Number(item.id) === Number(id))
  return registry ? `${registry.name} · ${registry.address}${registry.namespace ? `/${registry.namespace}` : ''}` : apt('selectRegistryPrompt')
}

// 원본 :293-298
function createBlankPipeline() {
  resetForm()
  props.page.form.stages = []
  props.page.templateVisible = false
  props.page.editorVisible = true
}

// 원본 :300-302
function useTemplate(template) {
  props.page.selectedTemplate = template
}

// 원본 :304-317
function confirmTemplate() {
  if (!props.page.selectedTemplate) {
    ElMessage.warning(apt('selectTemplateWarning'))
    return
  }
  resetForm()
  props.page.form.name = props.page.selectedTemplate.name.replace('General Template', uiT('pipeline'))
  props.page.form.techStack = props.page.selectedTemplate.techStack || 'custom'
  props.page.form.templateId = props.page.selectedTemplate.id
  props.page.form.description = props.page.selectedTemplate.description || ''
  props.page.form.stages = props.page.parseStages(props.page.selectedTemplate.definitionJson)
  props.page.templateVisible = false
  props.page.editorVisible = true
}

// 원본 :338-342
function fillFromApp() {
  if (!props.page.currentApp) return
  if (!props.page.form.defaultBranch) props.page.form.defaultBranch = props.page.currentApp.branch || 'master'
  if (!props.page.form.env) props.page.form.env = props.page.currentApp.env || 'test'
}

// 원본 :344-346
function addStage(type) {
  props.page.form.stages.push(defaultStage(type))
}

// 원본 :348-350
function removeStage(index) {
  props.page.form.stages.splice(index, 1)
}

// 원본 :352-357
function moveStage(index, direction) {
  const target = index + direction
  if (target < 0 || target >= props.page.form.stages.length) return
  const [stage] = props.page.form.stages.splice(index, 1)
  props.page.form.stages.splice(target, 0, stage)
}

// 원본 :374-402
async function submitPipeline() {
  const form = props.page.form
  if (!form.name || !form.appId) {
    ElMessage.warning(apt('nameAndAppRequired'))
    return
  }
  const stageError = props.page.validateStages()
  if (stageError) return ElMessage.warning(stageError)
  props.page.saving = true
  try {
    await saveOpsAppPipeline({
      id: form.id,
      name: form.name,
      appId: form.appId,
      defaultBranch: form.defaultBranch,
      env: form.env,
      techStack: form.techStack,
      templateId: form.templateId,
      executorHostId: form.executorHostId,
      status: form.status,
      description: form.description,
      definitionJson: stringifyDefinition()
    })
    ElMessage.success(apt('saveSuccess'))
    props.page.editorVisible = false
    await props.page.loadData()
  } finally {
    props.page.saving = false
  }
}
</script>

<template>
  <el-dialog v-model="page.templateVisible" :title="apt('selectTemplateTitle')" width="980px" class="template-dialog">
    <div class="template-picker">
      <aside>
        <button v-for="item in categories" :key="item" :class="{ active: page.selectedCategory === item }" @click="page.selectedCategory = item">
          {{ categoryLabel(item) }}
        </button>
      </aside>
      <main>
        <div v-if="page.selectedCategory === 'blank'" class="blank-template" @click="createBlankPipeline">
          <strong>{{ apt('blankPipeline') }}</strong>
          <p>{{ apt('blankPipelineDesc') }}</p>
        </div>
        <div v-else class="template-grid">
          <div v-for="item in filteredTemplates" :key="item.id" class="template-card" :class="{ selected: page.selectedTemplate?.id === item.id }" @click="useTemplate(item)">
            <strong>{{ item.name }}</strong>
            <p>{{ item.description }}</p>
            <span>{{ apt('stageCountSuffix', { techStack: item.techStack, count: item.stageCount }) }}</span>
          </div>
        </div>
      </main>
    </div>
    <template #footer>
      <el-button @click="page.templateVisible = false">{{ apt('cancel') }}</el-button>
      <el-button @click="createBlankPipeline">{{ apt('blankPipeline') }}</el-button>
      <el-button type="primary" @click="confirmTemplate">{{ apt('useSelectedTemplate') }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="page.editorVisible" :title="page.form.id ? apt('editPipelineTitle') : apt('newPipeline')" width="1180px" class="pipeline-editor">
    <el-form label-width="100px">
      <div class="form-grid">
        <el-form-item :label="apt('pipelineNameLabel')" required><el-input v-model="page.form.name" :placeholder="apt('pipelineNamePlaceholder')" /></el-form-item>
        <el-form-item :label="uiT('application')" required>
          <el-select v-model="page.form.appId" filterable :placeholder="apt('applicationPlaceholder')" @change="fillFromApp">
            <el-option v-for="item in page.appOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Default Branch"><el-input v-model="page.form.defaultBranch" :placeholder="apt('defaultBranchPlaceholder')" /></el-form-item>
        <el-form-item label="Default Environment">
          <el-select v-model="page.form.env">
            <el-option label="dev" value="dev" />
            <el-option label="test" value="test" />
            <el-option label="staging" value="staging" />
            <el-option label="prod" value="prod" />
          </el-select>
        </el-form-item>
        <el-form-item :label="uiT('techStack')">
          <el-select v-model="page.form.techStack">
            <el-option label="Go" value="go" />
            <el-option label="Maven Java" value="maven" />
            <el-option label="Vue" value="vue" />
            <el-option :label="apt('customTechStack')" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item :label="apt('executionNode')" required>
          <el-select v-model="page.form.executorHostId" filterable :placeholder="apt('executorHostPlaceholder')">
            <el-option v-for="host in page.executorHostOptions" :key="host.id" :label="page.executorHostLabel(host)" :value="host.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="apt('status')">
          <el-radio-group v-model="page.form.status">
            <el-radio :value="1">{{ apt('activate') }}</el-radio>
            <el-radio :value="2">{{ apt('deactivate') }}</el-radio>
          </el-radio-group>
        </el-form-item>
      </div>
      <el-alert type="warning" :closable="false" show-icon :title="apt('sshAlert')" />
      <el-form-item :label="apt('description')"><el-input v-model="page.form.description" type="textarea" :rows="2" :placeholder="apt('pipelineDescPlaceholder')" /></el-form-item>
    </el-form>

    <div class="stage-toolbar">
      <strong>Stage Orchestration</strong>
      <div>
        <el-button size="small" @click="addStage('checkout')">Source Checkout</el-button>
        <el-button size="small" @click="addStage('command')">Command</el-button>
        <el-button size="small" @click="addStage('test')">Test</el-button>
        <el-button size="small" @click="addStage('build')">Build</el-button>
        <el-button size="small" @click="addStage('dockerBuild')">Image Build</el-button>
        <el-button size="small" @click="addStage('dockerPush')">Image Registry Push</el-button>
        <el-button size="small" @click="addStage('k8sDeploy')">Kubernetes Deploy</el-button>
        <el-button size="small" @click="addStage('manual')">Manual Approval</el-button>
        <el-button size="small" @click="addStage('notify')">Notification</el-button>
      </div>
    </div>
    <div class="stage-editor">
      <el-empty v-if="!page.form.stages.length" :description="apt('emptyPipelineDesc')" />
      <div v-for="(stage, index) in page.form.stages" :key="stage.id" class="stage-config-card">
        <div class="stage-card-heading">
          <span class="stage-type-badge" :class="stageTone(stage.type)">{{ String(index + 1).padStart(2, '0') }}</span>
          <div>
            <strong>{{ page.stageTypeText(stage.type) }}</strong>
            <small>{{ stageHint(stage.type) }}</small>
          </div>
        </div>
        <div class="stage-order-actions">
          <el-tooltip :content="apt('moveStageUp')"><el-button circle size="small" :disabled="index === 0" @click="moveStage(index, -1)">↑</el-button></el-tooltip>
          <el-tooltip :content="apt('moveStageDown')"><el-button circle size="small" :disabled="index === page.form.stages.length - 1" @click="moveStage(index, 1)">↓</el-button></el-tooltip>
        </div>
        <div class="stage-fields stage-main-fields">
          <label><span>{{ apt('stageNameLabel') }}</span><el-input v-model="stage.name" :placeholder="apt('stageNamePlaceholder')" /></label>
          <label><span>Stage Type</span><el-select v-model="stage.type" @change="normalizeStageConfig(stage)">
            <el-option label="Source Checkout" value="checkout" />
            <el-option label="Command" value="command" />
            <el-option label="Test" value="test" />
            <el-option label="Build" value="build" />
            <el-option label="Image Build" value="dockerBuild" />
            <el-option label="Image Registry Push" value="dockerPush" />
            <el-option label="Kubernetes Deploy" value="k8sDeploy" />
            <el-option label="Manual Approval" value="manual" />
            <el-option label="Notification" value="notify" />
          </el-select></label>
          <label><span>Timeout</span><div class="stage-timeout"><el-input-number v-model="stage.timeoutSeconds" :min="10" :max="7200" controls-position="right" /><em>{{ apt('secondsUnit') }}</em></div></label>
          <label><span>Failure Policy</span><el-select v-model="stage.failurePolicy">
            <el-option :label="apt('failureStop')" value="stop" />
            <el-option :label="apt('failureIgnore')" value="ignore" />
          </el-select></label>
        </div>
        <div v-if="['command', 'test', 'build'].includes(stage.type)" class="script-stage-config">
          <div class="script-stage-title"><b>{{ apt('scriptSectionTitle') }}</b><span>{{ apt('scriptSectionDesc') }}</span></div>
          <el-input v-model="stage.config.script" type="textarea" :rows="5" :placeholder="apt('scriptPlaceholder')" />
        </div>
        <div v-else-if="stage.type === 'dockerBuild'" class="image-stage-config">
          <div class="image-stage-title"><span class="image-stage-badge build">01</span><div><strong>{{ apt('imageArtifactTitle') }}</strong><small>{{ apt('imageArtifactDesc') }}</small></div></div>
          <div class="image-stage-fields build-fields">
            <label><span>Target Image Registry</span><el-select v-model="stage.config.registryId" filterable :placeholder="apt('registrySelectPlaceholder')"><el-option v-for="registry in page.imageRegistryOptions" :key="registry.id" :label="`${registry.name} (${registry.address}${registry.namespace ? '/' + registry.namespace : ''})`" :value="registry.id" /></el-select></label>
            <label><span>Dockerfile</span><el-input v-model="stage.config.dockerfile" placeholder="Dockerfile" /></label>
            <label><span>Build Context</span><el-input v-model="stage.config.context" placeholder="." /></label>
          </div>
          <div class="image-stage-preview"><span>{{ apt('buildResult') }}</span><code>{{ registryName(stage.config.registryId) }} / {{ page.currentApp?.code || '<Application Code>' }} : {{ page.form.defaultBranch || '<Branch>' }}-{{ '{Time}' }}</code></div>
        </div>
        <div v-else-if="stage.type === 'dockerPush'" class="image-stage-config">
          <div class="image-stage-title"><span class="image-stage-badge push">02</span><div><strong>Build Image Push</strong><small>{{ apt('imagePushDesc') }}</small></div></div>
          <div class="image-stage-fields push-fields">
            <label><span>Image Source</span><el-select v-model="stage.config.sourceStageId" filterable :placeholder="apt('sourceStagePlaceholder')"><el-option v-for="buildStage in dockerBuildStages(stage.id)" :key="buildStage.id" :label="`${buildStage.name} · ${registryName(buildStage.config.registryId)}`" :value="buildStage.id" /></el-select></label>
            <label><span>{{ apt('loginModeLabel') }}</span><el-radio-group v-model="stage.config.loginMode" class="login-mode-group"><el-radio-button value="registry">Image Registry Credential</el-radio-button><el-radio-button value="executor">{{ apt('executorLoginOption') }}</el-radio-button></el-radio-group></label>
          </div>
          <div class="image-stage-tip"><span>✓</span><p><b>{{ apt('credentialTipTitle') }}</b>: {{ apt('credentialTipBody') }}</p></div>
        </div>
        <div v-else-if="stage.type === 'k8sDeploy'" class="delivery-stage-config">
          <div class="delivery-stage-title"><b>Deploy Target</b><span>{{ apt('deployTargetDesc') }}</span></div>
          <div class="stage-config-grid">
          <label><span>Kubernetes Cluster</span><el-select v-model="stage.config.clusterId" filterable :placeholder="apt('clusterSelectPlaceholder')">
            <el-option
              v-for="cluster in page.k8sClusterOptions"
              :key="cluster.id"
              :label="`${cluster.name} (${cluster.statusText || cluster.status || '-'} / ${cluster.version || '-'})`"
              :value="cluster.id"
            />
          </el-select></label>
          <label><span>Workload Type</span><el-select v-model="stage.config.workloadType" placeholder="Workload Type">
            <el-option label="Deployment" value="deployment" />
            <el-option label="StatefulSet" value="statefulset" />
            <el-option label="DaemonSet" value="daemonset" />
          </el-select></label>
          <label><span>Namespace</span><el-input v-model="stage.config.namespace" :placeholder="apt('namespacePlaceholder')" /></label>
          <label><span>{{ apt('workloadNameLabel') }}</span><el-input v-model="stage.config.workload" :placeholder="apt('workloadPlaceholder')" /></label>
          <label><span>{{ apt('containerNameLabel') }}</span><el-input v-model="stage.config.container" :placeholder="apt('containerPlaceholder')" /></label>
          <label><span>Image Registry Address</span><el-input v-model="stage.config.repository" :placeholder="apt('repositoryPlaceholder')" /></label>
          <label class="wide"><span>{{ apt('healthUrlLabel') }}</span><el-input v-model="stage.config.healthUrl" :placeholder="apt('healthUrlPlaceholder')" /></label>
          </div>
        </div>
        <div v-else-if="stage.type === 'checkout'" class="stage-note source-note"><b>Source</b><p>{{ apt('sourceNoteBody') }}</p></div>
        <div v-else-if="stage.type === 'manual'" class="stage-note manual-note"><b>Manual Approval Gate</b><p>{{ apt('manualNoteBody') }}</p></div>
        <div v-else-if="stage.type === 'notify'" class="stage-notify-config">
          <div class="stage-note notify-note"><b>{{ apt('notifyNoteTitle') }}</b><p>{{ apt('notifyNoteBody') }}</p></div>
          <label><span>Notification Rule</span><el-select v-model="stage.config.notifyRuleId" filterable :placeholder="apt('notifyRulePlaceholder')">
            <el-option v-for="rule in page.notifyRuleOptions" :key="rule.id" :label="apt('channelCount', { name: rule.name, count: rule.channelIds?.length || 0 })" :value="rule.id" />
          </el-select></label>
        </div>
        <div class="stage-card-footer">
          <span>{{ apt('stageOrder', { current: index + 1, total: page.form.stages.length }) }}</span>
          <el-button link type="danger" @click="removeStage(index)">{{ apt('deleteStage') }}</el-button>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="page.editorVisible = false">{{ apt('cancel') }}</el-button>
      <el-button type="primary" :loading="page.saving" @click="submitPipeline">{{ apt('save') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 원본 :1000-1004 */
.template-picker { display: grid; grid-template-columns: 180px 1fr; min-height: 520px; border-top: 1px solid #e5edf8; border-bottom: 1px solid #e5edf8; }
.template-picker aside { padding: 16px 12px; background: #f5f8fc; border-right: 1px solid #e5edf8; }
.template-picker aside button { display: block; width: 100%; height: 40px; padding: 0 14px; border: 0; border-radius: 6px; background: transparent; color: #49617f; text-align: left; cursor: pointer; }
.template-picker aside button.active { background: #e7f0ff; color: #1677ff; font-weight: 700; }
.template-picker main { padding: 22px; }

/* 원본 :1005·:1007-1011 — 탭(부모 잔류)과 공유 복사 6건 */
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(280px, 1fr)); gap: 16px; }
.template-card, .blank-template { min-height: 140px; padding: 20px; border: 1px solid #d7e4f5; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card.selected, .template-card:hover, .blank-template:hover { border-color: #2f6be6; box-shadow: 0 8px 24px rgba(47, 107, 230, .12); }
.template-card strong, .blank-template strong { color: #10213d; font-size: 16px; }
.template-card p, .blank-template p { color: #6b7c9b; }
.template-card span { color: #1677ff; font-weight: 700; }

/* 원본 :1012-1029 */
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(320px, 1fr)); column-gap: 28px; }
.stage-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 18px 0 12px; padding: 16px; border: 1px solid #e5edf9; border-radius: 12px; background: #f8fbff; }
.stage-toolbar strong { color: #1b3760; font-size: 15px; }
.stage-toolbar > div { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 7px; }
.stage-editor { max-height: 520px; overflow: auto; padding: 2px 6px 2px 0; }
.stage-config-card { position: relative; margin-bottom: 14px; padding: 16px 18px; border: 1px solid #dfe9f8; border-radius: 12px; background: linear-gradient(135deg, #fff 0%, #f9fbff 100%); box-shadow: 0 4px 14px rgba(48, 76, 122, .035); }
.stage-card-heading { display: flex; align-items: center; gap: 10px; min-height: 34px; margin-bottom: 14px; padding-right: 88px; }
.stage-card-heading strong { display: block; color: #18355d; font-size: 14px; }
.stage-card-heading small { display: block; margin-top: 3px; color: #8291a8; font-size: 12px; }
.stage-type-badge { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: #fff; font-size: 12px; font-weight: 800; box-shadow: 0 4px 10px rgba(52, 95, 180, .2); }
.stage-type-badge.source { background: linear-gradient(135deg, #2d83ee, #4f68e9); }.stage-type-badge.command { background: linear-gradient(135deg, #5a6fe8, #7c5ce5); }.stage-type-badge.test { background: linear-gradient(135deg, #21aa91, #3bc28d); }.stage-type-badge.build { background: linear-gradient(135deg, #e09a37, #f3b646); }.stage-type-badge.image { background: linear-gradient(135deg, #3a72ed, #5d5fe9); }.stage-type-badge.push { background: linear-gradient(135deg, #16a582, #2abb91); }.stage-type-badge.deploy { background: linear-gradient(135deg, #2b9ce9, #1bb6be); }.stage-type-badge.manual { background: linear-gradient(135deg, #9664de, #c46bf0); }.stage-type-badge.notify { background: linear-gradient(135deg, #e9796c, #ed9a5a); }
.stage-order-actions { position: absolute; top: 16px; right: 16px; display: flex; gap: 5px; }
.stage-card-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 12px; padding-top: 10px; border-top: 1px dashed #e2eaf5; color: #8a9ab3; font-size: 12px; }
.stage-fields { display: grid; gap: 12px; margin-bottom: 12px; }
.stage-main-fields { grid-template-columns: 1.35fr 1fr 160px 150px; }
.stage-fields label, .stage-config-grid label, .stage-notify-config label { display: flex; min-width: 0; flex-direction: column; gap: 6px; color: #637895; font-size: 12px; font-weight: 600; }
.stage-fields :deep(.el-select), .stage-config-grid :deep(.el-select), .stage-notify-config :deep(.el-select) { width: 100%; }

/* 원본 :1029(.stage-timeout)·:1030-1033 */
.stage-timeout { display: flex; align-items: center; gap: 8px; }.stage-timeout :deep(.el-input-number) { width: 118px; }.stage-timeout em { color: #8796ac; font-style: normal; font-weight: 400; }
.script-stage-config, .delivery-stage-config, .stage-notify-config { padding: 13px 15px; border: 1px solid #e1eaf7; border-radius: 10px; background: #fff; }
.script-stage-title, .delivery-stage-title { display: flex; align-items: baseline; gap: 10px; margin-bottom: 10px; }.script-stage-title b, .delivery-stage-title b { color: #25436b; font-size: 13px; }.script-stage-title span, .delivery-stage-title span { color: #8796ad; font-size: 12px; }
.stage-config-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 12px; }.stage-config-grid .wide { grid-column: 1 / -1; }
.stage-note { padding: 13px 15px; border: 1px solid #dce8fb; border-radius: 10px; background: #f7faff; }.stage-note b { color: #305d99; font-size: 13px; }.stage-note p { margin: 5px 0 0; color: #7185a3; font-size: 12px; line-height: 1.65; }.manual-note { border-color: #ebe0fb; background: #fbf8ff; }.manual-note b { color: #7a58af; }.notify-note { margin-bottom: 12px; border-color: #fee5d7; background: #fffaf6; }.notify-note b { color: #b56b43; }

/* 원본 :1034-1054 */
.image-stage-config { padding: 14px 16px; border: 1px solid #dfe8f7; border-radius: 10px; background: linear-gradient(135deg, #f9fbff 0%, #f5f8ff 100%); }
.image-stage-title { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.image-stage-title strong { display: block; color: #223b64; font-size: 14px; line-height: 1.4; }
.image-stage-title small { display: block; margin-top: 2px; color: #8191aa; font-size: 12px; }
.image-stage-badge { display: grid; width: 29px; height: 29px; place-items: center; border-radius: 8px; color: #fff; font-size: 12px; font-weight: 800; }
.image-stage-badge.build { background: linear-gradient(135deg, #3a72ed, #5d5fe9); box-shadow: 0 4px 10px rgba(58, 114, 237, .24); }
.image-stage-badge.push { background: linear-gradient(135deg, #16a582, #2abb91); box-shadow: 0 4px 10px rgba(22, 165, 130, .22); }
.image-stage-fields { display: grid; gap: 12px; }
.image-stage-fields.build-fields { grid-template-columns: minmax(260px, 1.65fr) minmax(150px, .7fr) minmax(120px, .55fr); }
.image-stage-fields.push-fields { grid-template-columns: minmax(280px, 1.4fr) minmax(310px, 1fr); }
.image-stage-fields label { display: flex; min-width: 0; flex-direction: column; gap: 6px; }
.image-stage-fields label > span { color: #637895; font-size: 12px; font-weight: 600; }
.image-stage-fields :deep(.el-select) { width: 100%; }
.image-stage-preview { display: flex; align-items: center; gap: 8px; margin-top: 12px; padding: 9px 11px; border-radius: 7px; background: #eef4ff; color: #7185a2; font-size: 12px; }
.image-stage-preview code { overflow: hidden; color: #315fc4; text-overflow: ellipsis; white-space: nowrap; font-family: Consolas, Monaco, monospace; font-size: 12px; }
.login-mode-group { display: flex; width: 100%; }
.login-mode-group :deep(.el-radio-button) { flex: 1; }
.login-mode-group :deep(.el-radio-button__inner) { width: 100%; padding: 9px 8px; }
.image-stage-tip { display: flex; gap: 8px; margin-top: 12px; padding: 9px 11px; border-radius: 7px; background: #f0fbf6; color: #56746c; font-size: 12px; line-height: 1.55; }
.image-stage-tip > span { display: grid; flex: 0 0 auto; width: 17px; height: 17px; place-items: center; border-radius: 50%; background: #20b485; color: #fff; font-size: 11px; font-weight: 800; }
.image-stage-tip p { margin: 0; }

/* 원본 :1055 */
@media (max-width: 900px) { .stage-toolbar { align-items: flex-start; flex-direction: column; }.stage-toolbar > div { justify-content: flex-start; }.stage-main-fields, .image-stage-fields.build-fields, .image-stage-fields.push-fields, .stage-config-grid { grid-template-columns: 1fr; }.stage-config-grid .wide { grid-column: auto; } }
</style>
