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

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    scriptType: 'shell',
    interpreter: 'bash',
    content: '',
    defaultParams: '',
    timeoutSeconds: 300,
    status: 1,
    description: '',
    changeSummary: ''
  })
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
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    scriptType: data.scriptType || 'shell',
    interpreter: data.interpreter || 'bash',
    content: data.content || '',
    defaultParams: data.defaultParams || '',
    timeoutSeconds: data.timeoutSeconds || 300,
    status: data.status || 1,
    description: data.description || '',
    changeSummary: ''
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入脚本名称')
    return
  }
  if (!form.content.trim()) {
    ElMessage.warning('请输入脚本内容')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await updateOpsScript(form)
      ElMessage.success('脚本已更新')
    } else {
      await addOpsScript(form)
      ElMessage.success('脚本已创建')
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
  ElMessage.success(nextStatus === 1 ? '脚本已启用' : '脚本已禁用')
  await loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除脚本“${row.name}”吗？`, '提示', { type: 'warning' })
  await deleteOpsScript(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

async function openVersions(row) {
  versionScript.value = row
  versionVisible.value = true
  versionLoading.value = true
  try { versionList.value = await queryOpsScriptVersions(row.id) } finally { versionLoading.value = false }
}

async function handleRollback(row) {
  await ElMessageBox.confirm(`确认将脚本回滚到 v${row.version}？回滚会生成一个新版本。`, '回滚脚本', { type: 'warning' })
  await rollbackOpsScript({ id: versionScript.value.id, version: row.version })
  ElMessage.success('脚本已回滚')
  versionList.value = await queryOpsScriptVersions(versionScript.value.id)
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">脚本库</h2>
        <p class="page-desc">集中维护常用运维脚本，支持启用禁用、默认参数和解释器配置。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增脚本</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="搜索脚本名称 / 描述" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.status" clearable placeholder="状态" style="width: 140px">
          <el-option label="启用" value="1" />
          <el-option label="禁用" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">搜索</el-button>
        <el-button @click="resetQuery">重置</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="name" label="脚本名称" min-width="180" />
      <el-table-column prop="scriptType" label="类型" width="120" />
      <el-table-column prop="interpreter" label="解释器" width="120" />
      <el-table-column prop="defaultParams" label="默认参数" min-width="180" show-overflow-tooltip />
      <el-table-column prop="timeoutSeconds" label="超时(秒)" width="120" />
      <el-table-column label="版本" width="90"><template #default="{ row }">v{{ row.currentVersion || 1 }}</template></el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="light">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updateTime" label="更新时间" min-width="180" />
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="openVersions(row)">版本</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" @click="handleStatusChange(row)">
            {{ row.status === 1 ? '禁用' : '启用' }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑脚本' : '新增脚本'" width="980px">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="脚本名称" required>
              <el-input v-model="form.name" placeholder="例如：重启 nginx" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="脚本类型">
              <el-select v-model="form.scriptType" style="width: 100%">
                <el-option label="Shell" value="shell" />
                <el-option label="Python" value="python" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="解释器">
              <el-select v-model="form.interpreter" style="width: 100%">
                <el-option label="bash" value="bash" />
                <el-option label="sh" value="sh" />
                <el-option label="python3" value="python3" />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="24">
            <el-form-item label="脚本内容" required>
              <div class="script-editor-shell">
                <div class="script-editor-toolbar">
                  <div class="editor-badges">
                    <span class="editor-badge">{{ form.scriptType === 'python' ? 'Python' : 'Shell' }}</span>
                    <span class="editor-badge subtle">{{ form.interpreter }}</span>
                  </div>
                  <span class="editor-meta">共 {{ scriptLineNumbers.length }} 行</span>
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
                      placeholder="请输入可通过 SSH 执行的脚本内容"
                      @scroll="onEditorScroll"
                    />
                  </div>
                </div>
              </div>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="默认参数">
              <el-input v-model="form.defaultParams" placeholder="例如：--env prod --force" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="超时(秒)">
              <el-input-number v-model="form.timeoutSeconds" :min="30" :max="3600" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="描述">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
          <el-col v-if="isEdit" :span="24">
            <el-form-item label="变更说明"><el-input v-model="form.changeSummary" placeholder="简要说明本次脚本调整" /></el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
    <el-drawer v-model="versionVisible" :title="`${versionScript?.name || ''} - 版本历史`" size="62%">
      <el-table v-loading="versionLoading" :data="versionList" border>
        <el-table-column label="版本" width="90"><template #default="{ row }">v{{ row.version }}</template></el-table-column>
        <el-table-column prop="changeSummary" label="变更说明" min-width="180" />
        <el-table-column prop="operator" label="操作人" width="130"><template #default="{ row }">{{ row.operator || 'system' }}</template></el-table-column>
        <el-table-column prop="createTime" label="创建时间" width="190" />
        <el-table-column prop="content" label="脚本内容" min-width="300" show-overflow-tooltip />
        <el-table-column label="操作" width="100"><template #default="{ row }"><el-button link type="primary" @click="handleRollback(row)">回滚</el-button></template></el-table-column>
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
  background: transparent;
  color: transparent;
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
