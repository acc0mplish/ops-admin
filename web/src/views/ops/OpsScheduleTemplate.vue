<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryOpsScriptOptions, queryOpsScheduleTemplateList, addOpsScheduleTemplate, updateOpsScheduleTemplate, deleteOpsScheduleTemplate, opsScheduleTemplateInfo } from '../../api/ops'
import OpsCronEditor from './components/OpsCronEditor.vue'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const scriptOptions = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  taskType: '',
  status: ''
})

const form = reactive({
  id: undefined,
  name: '',
  taskType: 'script',
  scriptId: undefined,
  variables: {},
  httpMethod: 'GET',
  url: '',
  headersJson: '{\n  "User-Agent": "OpsAdmin-Scheduler"\n}',
  body: '',
  expectedStatus: 200,
  timeoutSeconds: 10,
  cronExpr: '0 */5 * * * *',
  description: '',
  status: 1
})

const selectedScriptTimeout = computed(() => {
  const current = scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId))
  return current?.timeoutSeconds || 300
})

const selectedScriptVariables = computed(() => {
  const current = scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId))
  return current?.variables || []
})

function syncScriptVariables(values = {}) {
  const next = {}
  selectedScriptVariables.value.forEach((variable) => {
    next[variable.name] = values[variable.name] ?? (variable.secret ? '' : (variable.defaultValue || ''))
  })
  form.variables = next
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    taskType: 'script',
    scriptId: undefined,
    variables: {},
    httpMethod: 'GET',
    url: '',
    headersJson: '{\n  "User-Agent": "OpsAdmin-Scheduler"\n}',
    body: '',
    expectedStatus: 200,
    timeoutSeconds: 10,
    cronExpr: '0 */5 * * * *',
    description: '',
    status: 1
  })
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsScheduleTemplateList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadScriptOptions() {
  scriptOptions.value = await queryOpsScriptOptions()
}

function resetQuery() {
  Object.assign(query, {
    pageNum: 1,
    pageSize: 10,
    keyword: '',
    taskType: '',
    status: ''
  })
  loadData()
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await opsScheduleTemplateInfo(row.id)
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    taskType: data.taskType || 'script',
    scriptId: data.scriptId || undefined,
    variables: data.variables || {},
    httpMethod: data.httpMethod || 'GET',
    url: data.url || '',
    headersJson: data.headersJson || '{}',
    body: data.body || '',
    expectedStatus: data.expectedStatus || 200,
    timeoutSeconds: data.timeoutSeconds || 10,
    cronExpr: data.cronExpr || '0 */5 * * * *',
    description: data.description || '',
    status: data.status || 1
  })
  syncScriptVariables(data.variables || {})
  dialogVisible.value = true
}

watch(() => form.scriptId, () => {
  if (form.taskType === 'script') syncScriptVariables(form.variables)
})

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('Template 이름을 입력하십시오.')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await updateOpsScheduleTemplate({ ...form })
      ElMessage.success('Template을 수정했습니다.')
    } else {
      await addOpsScheduleTemplate({ ...form })
      ElMessage.success('Template을 생성했습니다.')
    }
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`Template "${row.name}"을(를) 삭제하시겠습니까?`, '알림', { type: 'warning' })
  await deleteOpsScheduleTemplate(row.id)
  ElMessage.success('삭제했습니다.')
  await loadData()
}

function taskTypeLabel(value) {
  return value === 'http' ? 'HTTP Probe' : 'Script Task'
}

onMounted(async () => {
  await loadScriptOptions()
  await loadData()
})
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Task Template</h2>
        <p class="page-desc">재사용 가능한 Script Task Template과 HTTP Probe Template을 축적해 Schedule Task에 빠르게 적용합니다.</p>
      </div>
      <el-button type="primary" @click="openCreate">새 Template</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="Template 이름 / 설명 / 주소 검색" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable placeholder="Template 유형" style="width: 140px">
          <el-option label="Script Task" value="script" />
          <el-option label="HTTP Probe" value="http" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="상태" style="width: 120px">
          <el-option label="활성화" value="1" />
          <el-option label="비활성화" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="Template 이름" min-width="180" />
      <el-table-column label="Template 유형" width="120">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column prop="cronExpr" label="기본 Cron" width="160" />
      <el-table-column prop="scriptName" label="Script / 주소" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">{{ row.taskType === 'http' ? row.url : row.scriptName }}</template>
      </el-table-column>
      <el-table-column label="상태" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="설명" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" label="수정 시간" width="180" />
      <el-table-column label="작업" width="140" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">수정</el-button>
          <el-button link type="danger" @click="handleDelete(row)">삭제</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, sizes, prev, pager, next"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? 'Template 수정' : '새 Template'" width="min(1080px, 92vw)" class="schedule-template-dialog">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="24"><div class="form-section-title">기본 정보</div></el-col>
          <el-col :span="12">
            <el-form-item label="Template 이름" required>
              <el-input v-model="form.name" placeholder="예: 핵심 서비스 HTTP Health Check" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="Template 유형">
              <el-radio-group v-model="form.taskType">
                <el-radio value="script">Script Task</el-radio>
                <el-radio value="http">HTTP Probe</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="상태">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">활성화</el-radio>
                <el-radio :value="2">비활성화</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="설명">
              <el-input v-model="form.description" />
            </el-form-item>
          </el-col>
          <el-col :span="24"><div class="form-section-title">Schedule 계획</div></el-col>
          <el-col :span="24">
            <el-form-item label="기본 Cron" required>
              <OpsCronEditor v-model="form.cronExpr" />
            </el-form-item>
          </el-col>
          <el-col :span="24"><div class="form-section-title">실행 설정</div></el-col>

          <template v-if="form.taskType === 'script'">
            <el-col :span="12">
              <el-form-item label="Script" required>
                <el-select v-model="form.scriptId" filterable placeholder="Script Library에서 Script 선택" style="width: 100%">
                  <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Timeout(초)">
                <el-input :model-value="selectedScriptTimeout" disabled>
                  <template #append>s</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <div class="variable-panel">
                <div class="variable-panel__title">Template 변수</div>
                <div class="variable-panel__hint">저장한 뒤 변수는 Template과 함께 Task로 전달되며 <code>VARIABLE_변수명</code> 형태로 Script에 주입됩니다.</div>
                <div v-if="!selectedScriptVariables.length" class="variable-panel__empty">이 Script는 변수를 선언하지 않아 구성할 필요가 없습니다.</div>
                <div v-else class="variable-grid">
                  <div v-for="variable in selectedScriptVariables" :key="variable.name" class="variable-field">
                    <div class="variable-field__label"><code>VARIABLE_{{ variable.name }}</code><el-tag v-if="variable.required" size="small" type="danger" effect="plain">필수</el-tag></div>
                    <el-input v-model="form.variables[variable.name]" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :placeholder="variable.secret ? '민감 값은 저장 후 다시 표시되지 않습니다' : (variable.defaultValue || '변수 값을 입력하십시오.')" />
                    <div v-if="variable.description" class="variable-field__desc">{{ variable.description }}</div>
                  </div>
                </div>
              </div>
            </el-col>
          </template>

          <template v-else>
            <el-col :span="6">
              <el-form-item label="요청 Method">
                <el-select v-model="form.httpMethod" style="width: 100%">
                  <el-option label="GET" value="GET" />
                  <el-option label="POST" value="POST" />
                  <el-option label="PUT" value="PUT" />
                  <el-option label="PATCH" value="PATCH" />
                  <el-option label="DELETE" value="DELETE" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Probe 주소">
                <el-input v-model="form.url" placeholder="예: https://example.com/healthz" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="Timeout(초)">
                <el-input-number v-model="form.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="예상 Status Code">
                <el-input-number v-model="form.expectedStatus" :min="100" :max="599" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="18">
              <el-form-item label="요청 Header JSON">
                <el-input v-model="form.headersJson" type="textarea" :rows="4" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="요청 Body">
                <el-input v-model="form.body" type="textarea" :rows="5" />
              </el-form-item>
            </el-col>
          </template>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="saving" @click="submit">저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 22px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar-left { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.form-section-title {
  margin: 4px 0 16px;
  padding-left: 10px;
  border-left: 3px solid #3b73e8;
  color: #172744;
  font-size: 15px;
  font-weight: 700;
}
.variable-panel { padding: 16px; border: 1px solid #d9e6ff; border-radius: 10px; background: linear-gradient(135deg, #f8fbff, #fff); }
.variable-panel__title { color: #172744; font-size: 15px; font-weight: 700; }
.variable-panel__hint, .variable-field__desc { margin-top: 4px; color: #7282a0; font-size: 12px; }
.variable-panel code, .variable-field__label code { color: #3869d9; }
.variable-panel__empty { margin-top: 14px; padding: 12px; color: #8190aa; border: 1px dashed #cbdcff; border-radius: 7px; }
.variable-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.variable-field__label { display: flex; align-items: center; gap: 8px; margin-bottom: 7px; font-size: 13px; font-weight: 600; }
@media (max-width: 720px) { .variable-grid { grid-template-columns: 1fr; } }
</style>
