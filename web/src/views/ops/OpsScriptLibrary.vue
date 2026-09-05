<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addOpsScript,
  deleteOpsScript,
  opsScriptInfo,
  queryOpsScriptVersions,
  queryOpsScriptList,
  rollbackOpsScript,
  updateOpsScript,
  updateOpsScriptStatus
} from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const isEdit = ref(false)
const tableData = ref([])
const total = ref(0)
const scriptEditorRef = ref(null)
const scriptScrollTop = ref(0)
const scriptScrollLeft = ref(0)
const versionVisible = ref(false)
const versionLoading = ref(false)
const versionList = ref([])
const versionScript = ref(null)
const scriptTimeoutPresets = [
  { label: ot('timeoutPreset30s'), value: 30 },
  { label: ot('timeoutPreset1m'), value: 60 },
  { label: ot('timeoutPreset5m'), value: 300 },
  { label: ot('timeoutPreset10m'), value: 600 },
  { label: ot('timeoutPreset30m'), value: 1800 }
]

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: ''
})

const form = reactive({
  id: undefined,
  name: '',
  scriptType: 'shell',
  interpreter: 'bash',
  content: '',
  defaultParams: '',
  variables: [],
  timeoutSeconds: 300,
  status: 1,
  description: '',
  changeSummary: ''
})

const scriptLineNumbers = computed(() => {
  const totalLines = Math.max(1, (form.content || '').split('\n').length)
  return Array.from({ length: totalLines }, (_, index) => index + 1)
})

const highlightedScript = computed(() => highlightScript(form.content || '', form.scriptType))
const timeoutPreset = computed({
  get: () => scriptTimeoutPresets.some((item) => item.value === form.timeoutSeconds) ? form.timeoutSeconds : 'custom',
  set: (value) => {
    if (value !== 'custom') form.timeoutSeconds = Number(value)
  }
})
const timeoutDescription = computed(() => {
  const seconds = Number(form.timeoutSeconds) || 0
  if (seconds < 60) return ot('durationSecondsShort', { seconds })
  if (seconds % 60 === 0) return ot('durationMinutesShort', { minutes: seconds / 60 })
  return ot('durationMinutesSecondsShort', { minutes: Math.floor(seconds / 60), seconds: seconds % 60 })
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    scriptType: 'shell',
    interpreter: 'bash',
    content: '',
    defaultParams: '',
    variables: [],
    timeoutSeconds: 300,
    status: 1,
    description: '',
    changeSummary: ''
  })
}

function compatibleInterpreter(scriptType, interpreter) {
  const allowed = scriptType === 'python' ? ['python', 'python3'] : ['bash', 'sh']
  return allowed.includes(interpreter) ? interpreter : (scriptType === 'python' ? 'python' : 'bash')
}

function handleScriptTypeChange(scriptType) {
  form.interpreter = scriptType === 'python' ? 'python' : 'bash'
}

function addVariable() {
  form.variables.push({ name: '', defaultValue: '', description: '', required: false, secret: false })
}

function removeVariable(index) { form.variables.splice(index, 1) }

function normalizeVariableName(variable) {
  variable.name = String(variable.name || '').toUpperCase().replace(/[^A-Z0-9_]/g, '')
}

watch(
  () => dialogVisible.value,
  (visible) => {
    if (visible) {
      nextTick(() => {
        scriptEditorRef.value?.focus()
      })
    } else {
      scriptScrollTop.value = 0
      scriptScrollLeft.value = 0
    }
  }
)

function onEditorScroll(event) {
  scriptScrollTop.value = event.target.scrollTop || 0
  scriptScrollLeft.value = event.target.scrollLeft || 0
}

function escapeHTML(value) {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function highlightCommon(source) {
  return source
    .replace(/\b(true|false|null)\b/g, '<span class="token builtin">$1</span>')
    .replace(/\b\d+(\.\d+)?\b/g, '<span class="token number">$&</span>')
}

function highlightShell(source) {
  let html = escapeHTML(source)
  html = html.replace(/(^|\s)(#[^\n]*)/gm, '$1<span class="token comment">$2</span>')
  html = html.replace(/("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')/g, '<span class="token string">$1</span>')
  html = html.replace(/\$(\w+|\{[^}]+\})/g, '<span class="token variable">$$$1</span>')
  html = html.replace(/\b(function|if|then|else|elif|fi|for|in|do|done|case|esac|while|export|return|exit)\b/g, '<span class="token keyword">$1</span>')
  html = html.replace(/\b(echo|grep|awk|sed|curl|scp|ssh|tar|systemctl|kubectl|python3?|bash|sh)\b/g, '<span class="token command">$1</span>')
  return highlightCommon(html)
}

function highlightPython(source) {
  let html = escapeHTML(source)
  html = html.replace(/(^|\s)(#[^\n]*)/gm, '$1<span class="token comment">$2</span>')
  html = html.replace(/("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')/g, '<span class="token string">$1</span>')
  html = html.replace(/\b(def|class|if|elif|else|for|while|in|return|import|from|as|try|except|finally|with|pass|break|continue|lambda)\b/g, '<span class="token keyword">$1</span>')
  html = html.replace(/\b(print|len|range|open|json|dict|list|str|int|float|bool)\b/g, '<span class="token command">$1</span>')
  return highlightCommon(html)
}

function highlightScript(source, scriptType) {
  if (!source) return ''
  return scriptType === 'python' ? highlightPython(source) : highlightShell(source)
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsScriptList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, {
    pageNum: 1,
    pageSize: 10,
    keyword: '',
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
  const data = await opsScriptInfo(row.id)
  const scriptType = data.scriptType || 'shell'
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    scriptType,
    interpreter: compatibleInterpreter(scriptType, data.interpreter),
    content: data.content || '',
    defaultParams: data.defaultParams || '',
    variables: (data.variables || []).map((item) => ({ name: item.name || '', defaultValue: item.secret ? '' : (item.defaultValue || ''), description: item.description || '', required: Boolean(item.required), secret: Boolean(item.secret) })),
    timeoutSeconds: data.timeoutSeconds || 300,
    status: data.status || 1,
    description: data.description || '',
    changeSummary: ''
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning(ot('scriptNameRequired'))
    return
  }
  if (!form.content.trim()) {
    ElMessage.warning(ot('scriptContentRequired'))
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await updateOpsScript(form)
      ElMessage.success(ot('scriptUpdated'))
    } else {
      await addOpsScript(form)
      ElMessage.success(ot('scriptCreated'))
    }
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleStatusChange(row) {
  const nextStatus = row.status === 1 ? 2 : 1
  await updateOpsScriptStatus({ id: row.id, status: nextStatus })
  ElMessage.success(nextStatus === 1 ? ot('scriptEnabledMessage') : ot('scriptDisabledMessage'))
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(ot('deleteScriptConfirm', { name: row.name }), ot('noticeTitle'), { type: 'warning' })
  await deleteOpsScript(row.id)
  ElMessage.success(ot('deleteSuccess'))
  await loadData()
}

async function openVersions(row) {
  versionScript.value = row
  versionVisible.value = true
  versionLoading.value = true
  try { versionList.value = await queryOpsScriptVersions(row.id) } finally { versionLoading.value = false }
}

async function handleRollback(row) {
  await ElMessageBox.confirm(ot('rollbackScriptConfirm', { version: row.version }), 'Script Rollback', { type: 'warning' })
  await rollbackOpsScript({ id: versionScript.value.id, version: row.version })
  ElMessage.success(ot('scriptRollbackSuccess'))
  versionList.value = await queryOpsScriptVersions(versionScript.value.id)
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Script Library</h2>
        <p class="page-desc">{{ ot('scriptLibraryDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ ot('newScript') }}</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchScript')" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.status" clearable :placeholder="ot('status')" style="width: 140px">
          <el-option :label="ot('enabled')" value="1" />
          <el-option :label="ot('disabled')" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="name" :label="ot('scriptName')" min-width="180" />
      <el-table-column prop="scriptType" :label="ot('type')" width="120" />
      <el-table-column prop="interpreter" label="Interpreter" width="120" />
      <el-table-column prop="defaultParams" :label="ot('defaultParameters')" min-width="180" show-overflow-tooltip />
      <el-table-column prop="timeoutSeconds" :label="ot('timeoutSecondsLabel')" width="120" />
      <el-table-column label="Version" width="90"><template #default="{ row }">v{{ row.currentVersion || 1 }}</template></el-table-column>
      <el-table-column :label="ot('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'warning'" effect="light" :class="{ 'script-status-disabled': row.status !== 1 }">
            {{ row.status === 1 ? ot('enabled') : ot('disabled') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="ot('description')" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" :label="ot('updateTime')" min-width="180" />
      <el-table-column :label="ot('actions')" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ ot('edit') }}</el-button>
          <el-button link type="primary" @click="openVersions(row)">Version</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" :class="{ 'disable-action': row.status === 1 }" @click="handleStatusChange(row)">
            {{ row.status === 1 ? ot('disabled') : ot('enabled') }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ ot('delete') }}</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? ot('editScript') : ot('newScript')" width="980px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="ot('scriptName')" required>
              <el-input v-model="form.name" :placeholder="ot('scriptNameExample')" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item :label="ot('scriptTypeLabel')">
              <el-select v-model="form.scriptType" style="width: 100%" @change="handleScriptTypeChange">
                <el-option label="Shell" value="shell" />
                <el-option label="Python" value="python" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="Interpreter">
              <el-select v-model="form.interpreter" style="width: 100%">
                <template v-if="form.scriptType === 'python'">
                  <el-option :label="ot('interpreterDefaultPython')" value="python" />
                  <el-option label="python3" value="python3" />
                </template>
                <template v-else>
                  <el-option :label="ot('interpreterDefaultBash')" value="bash" />
                  <el-option label="sh" value="sh" />
                </template>
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item :label="ot('scriptContent')" required>
              <div class="script-editor-shell">
                <div class="script-editor-toolbar">
                  <div class="editor-badges">
                    <span class="editor-badge">{{ form.scriptType === 'python' ? 'Python' : 'Shell' }}</span>
                    <span class="editor-badge subtle">{{ form.interpreter }}</span>
                  </div>
                  <span class="editor-meta">{{ ot('totalLines', { count: scriptLineNumbers.length }) }}</span>
                </div>
                <div class="script-editor-body">
                  <div class="script-gutter">
                    <div class="script-gutter-inner" :style="{ transform: `translateY(-${scriptScrollTop}px)` }">
                      <span v-for="line in scriptLineNumbers" :key="line">{{ line }}</span>
                    </div>
                  </div>
                  <div class="script-editor-stage">
                    <pre class="script-highlight" :style="{ transform: `translate(${-scriptScrollLeft}px, ${-scriptScrollTop}px)` }" v-html="highlightedScript + '\n'"></pre>
                    <textarea
                      ref="scriptEditorRef"
                      v-model="form.content"
                      class="script-editor"
                      spellcheck="false"
                      :placeholder="ot('scriptContentPlaceholder')"
                      @scroll="onEditorScroll"
                    />
                  </div>
                </div>
              </div>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <div class="variables-panel">
              <div class="variables-heading"><div><h3>Build Parameter</h3><p>{{ ot('buildParameterHintStart') }}<code>$VARIABLE_NAME</code>{{ ot('buildParameterHintOr') }}<code>${VARIABLE_NAME}</code>{{ ot('buildParameterHintEnd') }}</p></div><el-button type="primary" @click="addVariable">{{ ot('newBuildParameter') }}</el-button></div>
              <div v-if="!form.variables.length" class="variables-empty">{{ ot('noBuildParameters') }}</div>
              <div v-else class="variables-list">
                <div v-for="(variable, index) in form.variables" :key="index" class="variable-row">
                  <div class="variable-name"><span>VARIABLE_</span><el-input v-model="variable.name" placeholder="ENV" @input="normalizeVariableName(variable)" /></div>
                  <el-input v-model="variable.defaultValue" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :disabled="variable.secret" :placeholder="variable.secret ? ot('secretVariableNoDefault') : ot('defaultValueOptional')" />
                  <el-input v-model="variable.description" :placeholder="ot('descriptionOptional')" />
                  <el-checkbox v-model="variable.required">{{ ot('required') }}</el-checkbox>
                  <el-checkbox v-model="variable.secret" @change="variable.secret && (variable.defaultValue = '')">Secret</el-checkbox>
                  <el-button link type="danger" @click="removeVariable(index)">{{ ot('delete') }}</el-button>
                </div>
              </div>
            </div>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="ot('executionTimeout')">
              <div class="timeout-control">
                <el-select v-model="timeoutPreset" class="timeout-preset" :aria-label="ot('commonTimeouts')">
                  <el-option v-for="item in scriptTimeoutPresets" :key="item.value" :label="item.label" :value="item.value" />
                  <el-option :label="ot('custom')" value="custom" />
                </el-select>
                <el-input-number v-model="form.timeoutSeconds" :min="30" :max="3600" :step="30" controls-position="right" :aria-label="ot('timeoutSecondsLabel')" />
              </div>
              <div class="timeout-tip">{{ timeoutDescription }} · {{ ot('maxDurationHint') }}</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="ot('status')">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">{{ ot('enabled') }}</el-radio>
                <el-radio :value="2">{{ ot('disabled') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item :label="ot('description')">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
          <el-col v-if="isEdit" :span="24">
            <el-form-item :label="ot('changeSummaryLabel')"><el-input v-model="form.changeSummary" :placeholder="ot('changeSummaryPlaceholder')" /></el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ ot('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ ot('save') }}</el-button>
      </template>
    </el-dialog>
    <el-drawer v-model="versionVisible" :title="`${versionScript?.name || ''} - Version History`" size="62%">
      <el-table v-loading="versionLoading" :data="versionList" border>
        <el-table-column label="Version" width="90"><template #default="{ row }">v{{ row.version }}</template></el-table-column>
        <el-table-column prop="changeSummary" :label="ot('changeSummaryLabel')" min-width="180" />
        <el-table-column prop="operator" :label="ot('operator')" width="130"><template #default="{ row }">{{ row.operator || 'system' }}</template></el-table-column>
        <el-table-column prop="createTime" :label="ot('createdAt')" width="190" />
        <el-table-column prop="content" :label="ot('scriptContent')" min-width="300" show-overflow-tooltip />
        <el-table-column :label="ot('actions')" width="100"><template #default="{ row }"><el-button link type="primary" @click="handleRollback(row)">Rollback</el-button></template></el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<style scoped>
.ops-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 700;
  color: #14213d;
}

.page-desc {
  margin: 0;
  color: #7282a0;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.toolbar-left {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.pager {
  display: flex;
  justify-content: flex-end;
}

.script-status-disabled {
  font-weight: 700;
  letter-spacing: 0.04em;
}

.disable-action {
  color: #e6a23c !important;
  font-weight: 600;
}

.disable-action:hover,
.disable-action:focus-visible {
  color: #d68f24 !important;
}

.timeout-control {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px;
  width: 100%;
  overflow: hidden;
  border: 1px solid #d7dfec;
  border-radius: 8px;
  background: #fff;
}

.timeout-control:focus-within {
  border-color: #4f7df3;
  box-shadow: 0 0 0 3px rgba(79, 125, 243, 0.12);
}

.timeout-control :deep(.el-select__wrapper),
.timeout-control :deep(.el-input__wrapper) {
  min-height: 34px;
  box-shadow: none !important;
}

.timeout-preset {
  border-right: 1px solid #e3eaf4;
}

.timeout-tip {
  margin-top: 6px;
  color: #8090a8;
  font-size: 12px;
  line-height: 1.2;
}

.variables-panel { margin: 2px 0 18px; padding: 18px; border: 1px solid #d7e4fb; border-radius: 10px; background: linear-gradient(135deg, #fbfdff, #f5f8ff); }
.variables-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 14px; }.variables-heading h3 { margin: 0 0 4px; color: #172b4d; font-size: 16px; }.variables-heading p { margin: 0; color: #7184a2; font-size: 13px; }.variables-heading code { color: #456ee8; font-family: 'JetBrains Mono', Consolas, monospace; }
.variables-empty { padding: 16px 18px; border: 1px dashed #bdd2fa; border-radius: 7px; color: #7184a2; font-size: 13px; }
.variables-list { display: flex; flex-direction: column; gap: 9px; }.variable-row { display: grid; grid-template-columns: 1.25fr 1fr 1.2fr auto auto auto; align-items: center; gap: 9px; padding: 10px; border: 1px solid #e0e8f5; border-radius: 8px; background: #fff; }.variable-name { display: flex; align-items: center; overflow: hidden; border: 1px solid #d7dfec; border-radius: 6px; }.variable-name span { padding: 0 8px; color: #5473a5; font: 12px 'JetBrains Mono', Consolas, monospace; white-space: nowrap; }.variable-name :deep(.el-input__wrapper) { box-shadow: none; }.variable-row :deep(.el-checkbox) { margin-right: 0; white-space: nowrap; }
@media (max-width: 900px) { .variable-row { grid-template-columns: 1fr 1fr; }.variables-heading { align-items: flex-start; flex-direction: column; } }

.script-editor-shell {
  width: 100%;
  border: 1px solid #d7dfec;
  border-radius: 10px;
  overflow: hidden;
  background: #111827;
}

.script-editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: #0f172a;
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
}

.editor-badges {
  display: flex;
  gap: 8px;
}

.editor-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(96, 165, 250, 0.18);
  color: #bfdbfe;
  font-size: 12px;
  font-weight: 600;
}

.editor-badge.subtle {
  background: rgba(148, 163, 184, 0.14);
  color: #cbd5e1;
}

.editor-meta {
  color: #94a3b8;
  font-size: 12px;
}

.script-editor-body {
  display: flex;
  min-height: 420px;
}

.script-gutter {
  width: 58px;
  flex-shrink: 0;
  background: #0f172a;
  border-right: 1px solid rgba(148, 163, 184, 0.16);
  overflow: hidden;
}

.script-gutter-inner {
  padding: 14px 0;
}

.script-gutter span {
  display: block;
  height: 24px;
  line-height: 24px;
  padding-right: 12px;
  text-align: right;
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  font-size: 13px;
  color: #64748b;
}

.script-editor-stage {
  position: relative;
  flex: 1;
  min-height: 420px;
  overflow: hidden;
}

.script-highlight,
.script-editor {
  margin: 0;
  padding: 14px 16px;
  min-height: 420px;
  width: 100%;
  box-sizing: border-box;
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  font-size: 14px;
  line-height: 24px;
  white-space: pre;
  tab-size: 2;
}

.script-highlight {
  position: absolute;
  inset: 0;
  overflow: hidden;
  color: #e5e7eb;
  pointer-events: none;
}

.script-editor {
  position: relative;
  z-index: 1;
  border: 0;
  resize: none;
  outline: none;
  /* Keep the raw input readable even if syntax highlighting cannot render. */
  background: #111827;
  color: #e5e7eb;
  caret-color: #f8fafc;
  overflow: auto;
}

.script-editor::placeholder {
  color: #64748b;
}

:deep(.token.comment) {
  color: #6b7280;
}

:deep(.token.keyword) {
  color: #f472b6;
  font-weight: 600;
}

:deep(.token.string) {
  color: #fde047;
}

:deep(.token.variable) {
  color: #a3e635;
}

:deep(.token.command) {
  color: #60a5fa;
}

:deep(.token.number) {
  color: #c084fc;
}

:deep(.token.builtin) {
  color: #34d399;
}
</style>
