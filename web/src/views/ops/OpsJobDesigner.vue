<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ot } from '../../utils/ops-i18n'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import {
  addOpsJob,
  addOpsJobTemplate,
  opsJobInfo,
  opsJobTemplateInfo,
  queryNotifyRuleOptions,
  queryOpsJobTemplateOptions,
  queryOpsScriptOptions,
  updateOpsJob,
  updateOpsJobTemplate
} from '../../api/ops'
// I5-J (i5-plan §3.8 row 8·CW-4) — 1,009행 분할 잔류본. 선택 단계 설정 패널은
// job-designer/OpsJobNodeForm.vue, X6 그래프 캔버스는 OpsJobCanvas.vue로 이동
// (원본 좌표는 커밋 메시지·자식 파일 헤더). 자식은 :page 주입(§5 #11 승계)·
// 캔버스 조작 진입점은 defineExpose 위임으로 보존한다.
import OpsJobNodeForm from './job-designer/OpsJobNodeForm.vue'
import OpsJobCanvas from './job-designer/OpsJobCanvas.vue'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const saving = ref(false)
const graphReady = ref(false)
const importDialogVisible = ref(false)
const importTemplateId = ref()
const selectedNodeId = ref('')
const selectedEdgeId = ref('')
const selectedCellIds = ref([])
const canvasRef = ref(null)

const scriptOptions = ref([])
const hostOptions = ref([])
const groupOptions = ref([])
const templateOptions = ref([])
const notifyRuleOptions = ref([])

const form = reactive({
  id: undefined,
  name: '',
  description: '',
  status: 1,
  templateId: undefined,
  notifyEnabled: false,
  notifyRuleId: undefined
})

const selectedNodeForm = reactive({
  id: '',
  type: 'script',
  label: '',
  config: {}
})

const isTemplateMode = computed(() => String(route.query.mode || '') === 'template')
const editorTitle = computed(() => (isTemplateMode.value ? ot('editJobTemplate') : 'Job Orchestration'))
const saveButtonText = computed(() => (isTemplateMode.value ? ot('saveTemplate') : ot('saveJob')))
const selectedCount = computed(() => selectedCellIds.value.length)

// 원본 :127-130 normalizeTargetIds — 노드 폼 자식(updateSelectedHostIds/
// updateSelectedGroupIds)과 캔버스 자식(normalizeNodeTargets)이 공유해
// 부모 귀속 유지
function normalizeTargetIds(value) {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map((item) => Number(item)).filter((item) => item > 0))]
}

// 원본 :550-558 addStep·:309-318 removeSelectedEdge·:560-593 removeSelectedNode·
// :607-611 clearCanvas는 캔버스 자식 귀속 — 팔레트·툴바 진입점은 위임 호출.
function addStep(type) {
  canvasRef.value?.addStep(type)
}

function removeSelectedEdge() {
  canvasRef.value?.removeSelectedEdge()
}

function removeSelectedNode() {
  canvasRef.value?.removeSelectedNode()
}

function clearCanvas() {
  canvasRef.value?.clearCanvas()
}

async function loadBaseOptions() {
  const [scripts, hosts, groups, templates, notifyRules] = await Promise.all([
    queryOpsScriptOptions(),
    queryAssetHostList({ pageNum: 1, pageSize: 1000 }),
    queryAssetHostGroupList(),
    queryOpsJobTemplateOptions(),
    queryNotifyRuleOptions({ scope: 'job' })
  ])
  scriptOptions.value = scripts || []
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
  templateOptions.value = templates || []
  notifyRuleOptions.value = notifyRules || []
}

async function loadCurrentRecord() {
  const id = Number(route.query.id || 0)
  if (!id) return
  loading.value = true
  try {
    const data = isTemplateMode.value ? await opsJobTemplateInfo(id) : await opsJobInfo(id)
    form.id = data.id
    form.name = data.name || ''
    form.description = data.description || ''
    form.status = data.status || 1
    form.templateId = data.templateId || undefined
    form.notifyEnabled = !!data.notifyEnabled
    form.notifyRuleId = data.notifyRuleId || undefined
    const definition = JSON.parse(data.definitionJson || '{"nodes":[],"edges":[]}')
    canvasRef.value?.loadDefinition(definition, data.graphJson || '')
  } finally {
    loading.value = false
  }
}

async function importTemplate() {
  if (!importTemplateId.value) {
    ElMessage.warning(ot('selectTemplateFirst'))
    return
  }
  const data = await opsJobTemplateInfo(importTemplateId.value)
  form.templateId = data.id
  if (!form.name.trim()) {
    form.name = data.name || ''
  }
  if (!form.description.trim()) {
    form.description = data.description || ''
  }
  const definition = JSON.parse(data.definitionJson || '{"nodes":[],"edges":[]}')
  canvasRef.value?.loadDefinition(definition, data.graphJson || '')
  importDialogVisible.value = false
  ElMessage.success(ot('templateImported'))
}

async function save() {
  if (!form.name.trim()) {
    ElMessage.warning(isTemplateMode.value ? ot('templateNameRequired') : ot('jobNameRequired'))
    return
  }
  const definition = canvasRef.value?.serializeDefinition()
  if (!definition.nodes.length) {
    ElMessage.warning(ot('addStepRequired'))
    return
  }
  const invalidNotifyNode = definition.nodes.find((node) => node.type === 'notify' && !node.config?.notifyRuleId)
  if (invalidNotifyNode) {
    ElMessage.warning(ot('notifyRuleRequiredForNotifyStep'))
    return
  }
  saving.value = true
  try {
    const payload = {
      id: form.id,
      name: form.name,
      description: form.description,
      status: form.status,
      templateId: form.templateId,
      notifyEnabled: false,
      notifyRuleId: undefined,
      graphJson: canvasRef.value?.getGraphJson(),
      definitionJson: JSON.stringify(definition)
    }
    if (isTemplateMode.value) {
      if (form.id) {
        await updateOpsJobTemplate(payload)
      } else {
        await addOpsJobTemplate(payload)
      }
      ElMessage.success(ot('jobTemplateSaved'))
      router.push('/ops/jobs/templates')
      return
    }
    if (form.id) {
      await updateOpsJob(payload)
    } else {
      await addOpsJob(payload)
    }
    ElMessage.success(ot('jobSaved'))
    router.push('/ops/jobs/list')
  } finally {
    saving.value = false
  }
}

// 원본 :106-121 watch(route.fullPath) — 폼 리셋은 부모, 그래프 클리어·선택 해제는
// 캔버스 자식 위임(graph.clearCells()·loadSelectedNode('') 대체).
watch(
  () => route.fullPath,
  async () => {
    form.id = undefined
    form.name = ''
    form.description = ''
    form.status = 1
    form.templateId = undefined
    form.notifyEnabled = false
    form.notifyRuleId = undefined
    canvasRef.value?.clearCells()
    canvasRef.value?.loadSelectedNode('')
    await loadCurrentRecord()
  }
)

// I5-J 자식 주입 번들 — reactive 래핑으로 ref/reactive가 언랩되어
// 자식 템플릿·스크립트의 page.x 재배선이 동작한다(§5 #11).
const page = reactive({
  loading,
  graphReady,
  selectedNodeId,
  selectedEdgeId,
  selectedCellIds,
  selectedNodeForm,
  scriptOptions,
  hostOptions,
  groupOptions,
  notifyRuleOptions,
  normalizeTargetIds
})

onMounted(async () => {
  await loadBaseOptions()
  await loadCurrentRecord()
})
</script>

<template>
  <div class="ops-job-page">
    <div class="page-card page-head">
      <div>
        <h2 class="page-title">{{ editorTitle }}</h2>
        <p class="page-desc">{{ ot('jobDesignerDesc') }}</p>
      </div>
      <div class="head-actions">
        <el-button @click="importDialogVisible = true">{{ ot('importJobTemplate') }}</el-button>
        <el-button @click="selectedEdgeId ? removeSelectedEdge() : removeSelectedNode()" :disabled="!selectedCount && !selectedNodeId && !selectedEdgeId">
          {{ selectedEdgeId ? ot('removeEdge') : (selectedCount > 1 ? ot('deleteSelectedItems', { count: selectedCount }) : ot('deleteSelectedStep')) }}
        </el-button>
        <el-button @click="clearCanvas" :disabled="!graphReady">{{ ot('clearCanvas') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ saveButtonText }}</el-button>
      </div>
    </div>

    <div class="page-card record-form">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item :label="isTemplateMode ? ot('templateName') : ot('jobName')" required>
              <el-input v-model="form.name" :placeholder="isTemplateMode ? ot('templateNameRequired') : ot('jobNameRequired')" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="ot('status')">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">{{ ot('enabled') }}</el-radio>
                <el-radio :value="2">{{ ot('disabled') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="!isTemplateMode">
            <el-form-item label="Source Template">
              <el-select v-model="form.templateId" clearable filterable :placeholder="ot('optionalLabel')">
                <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="ot('description')">
              <el-input v-model="form.description" type="textarea" :rows="2" :placeholder="ot('jobDescriptionPlaceholder')" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="designer-layout">
      <div class="page-card left-palette">
        <div class="panel-title">Step Library</div>
        <el-button class="palette-btn" @click="addStep('script')">{{ ot('scriptExecution') }}</el-button>
        <el-button class="palette-btn" @click="addStep('file')">File Distribution</el-button>
        <el-button class="palette-btn" @click="addStep('approval')">Manual Approval</el-button>
        <el-button class="palette-btn" @click="addStep('notify')">{{ ot('messageNotify') }}</el-button>
        <div class="panel-tip">{{ ot('dragNodeHint') }}</div>
      </div>

      <OpsJobCanvas ref="canvasRef" :page="page" />

      <OpsJobNodeForm :page="page" />
    </div>

    <el-dialog v-model="importDialogVisible" :title="ot('importJobTemplate')" width="520px">
      <el-form label-width="90px">
        <el-form-item label="Job Template">
          <el-select v-model="importTemplateId" filterable :placeholder="ot('selectTemplateToImport')" style="width: 100%">
            <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">{{ ot('cancel') }}</el-button>
        <el-button type="primary" @click="importTemplate">{{ ot('importAction') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ops-job-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 700;
  color: #14213d;
}

.page-desc {
  margin: 0;
  color: #7485a7;
}

.head-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.designer-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr) 380px;
  gap: 16px;
  min-height: 720px;
}

/* 원본 :955-961 합성 셀렉터 분해 — .left-palette 소속.
    * .canvas-panel은 OpsJobCanvas·.config-panel은 OpsJobNodeForm 자식 */
.left-palette {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.panel-title {
  font-size: 18px;
  font-weight: 700;
  color: #14213d;
}

.palette-btn {
  justify-content: flex-start;
  height: 44px;
  border-radius: 10px;
}

.panel-tip {
  font-size: 12px;
  line-height: 1.7;
  color: #7d8cad;
}

@media (max-width: 1440px) {
  .designer-layout {
    grid-template-columns: 200px minmax(0, 1fr) 340px;
  }
}
</style>
