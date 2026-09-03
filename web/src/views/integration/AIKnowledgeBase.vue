<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { computed, onMounted, reactive, ref } from 'vue'
import { Delete, Document, DocumentCopy, Edit, EditPen, Plus, Upload, View } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteAIKnowledgeDocument, queryAIKnowledgeDocuments, saveAIKnowledgeDocument, uploadAIKnowledgeDocument } from '../../api/integration'
import { it } from '../../utils/integration-i18n'
import './ai.css'

const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const dialogVisible = ref(false)
const editorMode = ref('edit')
const keyword = ref('')
const documents = ref([])
const form = reactive({ id: undefined, name: '', fileName: '', content: '', status: 1, sourceType: 'manual' })
const activeCount = computed(() => documents.value.filter((item) => item.status === 1).length)

function reset() { Object.assign(form, { id: undefined, name: '', fileName: '', content: '', status: 1, sourceType: 'manual' }) }
async function load() { loading.value = true; try { documents.value = (await queryAIKnowledgeDocuments({ keyword: keyword.value })) || [] } finally { loading.value = false } }
function createDocument() { reset(); editorMode.value = 'edit'; dialogVisible.value = true }
function editDocument(row) { Object.assign(form, { ...row, sourceType: row.sourceType || 'manual' }); editorMode.value = 'edit'; dialogVisible.value = true }
async function save() {
  if (!form.name.trim() || !form.content.trim()) return ElMessage.warning(it('documentRequired'))
  saving.value = true
  try { await saveAIKnowledgeDocument(form); dialogVisible.value = false; await load(); ElMessage.success(it('documentSaved')) } finally { saving.value = false }
}
async function remove(row) {
  await ElMessageBox.confirm(it('deleteDocumentConfirm', { name: row.name }), it('deleteDocument'), { type: 'warning' })
  await deleteAIKnowledgeDocument(row.id); await load(); ElMessage.success(it('documentDeleted'))
}
async function upload(file) {
  if (!file.name.toLowerCase().endsWith('.md')) { ElMessage.error(it('markdownOnly')); return false }
  if (file.size > 2 * 1024 * 1024) { ElMessage.error(it('markdownMax2mb')); return false }
  uploading.value = true
  try { const data = new FormData(); data.append('file', file); await uploadAIKnowledgeDocument(data); await load(); ElMessage.success(it('markdownImported')) } finally { uploading.value = false }
  return false
}
function preview(content) { return (content || '').replace(/\s+/g, ' ').slice(0, 140) || it('noContent') }
const contentLines = computed(() => (form.content || '').split('\n'))
onMounted(load)
</script>

<template>
  <div class="ai-page knowledge-page">
    <section class="ai-hero">
      <div><div class="ai-kicker">{{ uiT('localMarkdownKnowledge') }}</div><h1>{{ it('knowledgeManagement') }}</h1><p>{{ it('knowledgeManagementDesc') }}</p></div>
      <div class="knowledge-actions"><el-upload :show-file-list="false" :before-upload="upload" accept=".md"><el-button :loading="uploading" :icon="Upload">{{ it('uploadMarkdown') }}</el-button></el-upload><el-button type="primary" :icon="Plus" @click="createDocument">{{ it('newMarkdown') }}</el-button></div>
    </section>
    <section class="ai-panel">
      <div class="ai-panel-head knowledge-head"><div><h2>{{ it('localKnowledgeDocuments') }}</h2><span class="ai-muted">{{ it('activeDocuments', { active: activeCount, total: documents.length }) }}</span></div><el-input v-model="keyword" clearable :placeholder="it('searchDocumentName')" style="width: 240px" @keyup.enter="load" @clear="load"><template #append><el-button @click="load">{{ it('search') }}</el-button></template></el-input></div>
      <el-table v-loading="loading" :data="documents" :empty-text="it('noMarkdownDocuments')" style="width: 100%">
        <el-table-column :label="it('document')" min-width="260"><template #default="{ row }"><div class="doc-name"><el-icon><Document /></el-icon><div><strong>{{ row.name }}</strong><small>{{ row.fileName }}</small></div></div></template></el-table-column>
        <el-table-column :label="it('contentSummary')" min-width="380" show-overflow-tooltip><template #default="{ row }">{{ preview(row.content) }}</template></el-table-column>
        <el-table-column :label="it('source')" width="100"><template #default="{ row }"><el-tag size="small" effect="plain">{{ row.sourceType === 'upload' ? it('upload') : it('create') }}</el-tag></template></el-table-column>
        <el-table-column :label="it('status')" width="100"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? it('enabled') : it('disabled') }}</el-tag></template></el-table-column>
        <el-table-column prop="updateTime" :label="it('updatedAt')" min-width="170"/>
        <el-table-column :label="it('actions')" width="160"><template #default="{ row }"><div class="doc-actions"><el-button link type="primary" :icon="Edit" @click="editDocument(row)">{{ it('edit') }}</el-button><el-button link type="danger" :icon="Delete" @click="remove(row)">{{ it('delete') }}</el-button></div></template></el-table-column>
      </el-table>
    </section>
    <el-dialog v-model="dialogVisible" width="min(1120px, 92vw)" class="knowledge-editor-dialog" destroy-on-close>
      <template #header>
        <div class="editor-dialog-head"><span class="editor-dialog-icon"><el-icon><DocumentCopy /></el-icon></span><div><div class="ai-kicker">MARKDOWN KNOWLEDGE DOCUMENT</div><h2>{{ form.id ? it('editKnowledgeDocument') : it('newKnowledgeDocument') }}</h2><p>{{ it('knowledgeEditorDesc') }}</p></div><el-tag class="editor-source" effect="plain">{{ form.sourceType === 'upload' ? it('uploadedDocument') : it('onlineEdit') }}</el-tag></div>
      </template>
      <el-form class="knowledge-editor-form" label-position="top">
        <section class="editor-meta-card"><div class="editor-meta-title"><el-icon><EditPen /></el-icon><span>{{ it('documentInfo') }}</span></div><div class="document-form-grid"><el-form-item :label="it('documentName')" required><el-input v-model="form.name" :placeholder="it('documentNameExample')"/></el-form-item><el-form-item :label="it('fileName')"><el-input v-model="form.fileName" :placeholder="it('defaultFileName')"><template #append>.md</template></el-input></el-form-item></div></section>
        <section class="editor-workspace"><div class="editor-workspace-head"><div><strong>{{ it('markdownContent') }}</strong><span>{{ it('characters', { count: form.content.length.toLocaleString() }) }}</span></div><el-radio-group v-model="editorMode" size="small"><el-radio-button value="edit"><el-icon><EditPen /></el-icon> {{ it('edit') }}</el-radio-button><el-radio-button value="preview"><el-icon><View /></el-icon> {{ it('preview') }}</el-radio-button></el-radio-group></div><el-input v-if="editorMode === 'edit'" v-model="form.content" class="markdown-input" type="textarea" :rows="20" resize="none" :placeholder="it('markdownPlaceholder')"/><div v-else class="markdown-preview"><template v-for="(line, index) in contentLines" :key="index"><h1 v-if="line.startsWith('# ')" >{{ line.slice(2) }}</h1><h2 v-else-if="line.startsWith('## ')" >{{ line.slice(3) }}</h2><blockquote v-else-if="line.startsWith('> ')" >{{ line.slice(2) }}</blockquote><pre v-else-if="line.startsWith('```')" >{{ line }}</pre><p v-else>{{ line || ' ' }}</p></template></div></section>
        <section class="retrieval-setting"><div><strong>{{ it('aiRetrieval') }}</strong><span>{{ it('aiRetrievalDesc') }}</span></div><el-switch v-model="form.status" :active-value="1" :inactive-value="2" :active-text="it('enabled')" :inactive-text="it('disabled')"/></section>
      </el-form>
      <template #footer><div class="editor-footer"><span>{{ it('localDatabaseOnly') }}</span><div><el-button @click="dialogVisible = false">{{ it('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ it('saveDocument') }}</el-button></div></div></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.knowledge-actions { display: flex; gap: 10px; }.knowledge-head { align-items: center; }.doc-name { display: flex; align-items: center; gap: 10px; }.doc-name > .el-icon { display: grid; place-items: center; width: 34px; height: 34px; color: #356fe5; background: #eaf1ff; border-radius: 6px; }.doc-name strong, .doc-name small { display: block; }.doc-name small { margin-top: 3px; color: #8090a8; }.doc-actions { display: flex; align-items: center; gap: 14px; white-space: nowrap; }.doc-actions :deep(.el-button + .el-button) { margin-left: 0; }.document-form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }.document-form-grid :deep(.el-form-item) { margin-bottom: 0; }.editor-dialog-head { display: flex; align-items: center; gap: 12px; min-height: 42px; }.editor-dialog-head h2 { margin: 2px 0 3px; color: #122543; font-size: 21px; }.editor-dialog-head p { margin: 0; color: #8190a7; font-size: 13px; }.editor-dialog-icon { display: grid; place-items: center; width: 42px; height: 42px; color: #fff; background: linear-gradient(145deg, #3c78ed, #7058f5); border-radius: 9px; font-size: 21px; }.editor-source { margin-left: auto; }.knowledge-editor-form { display: grid; gap: 14px; }.editor-meta-card, .editor-workspace, .retrieval-setting { border: 1px solid #e1e9f5; border-radius: 9px; }.editor-meta-card { padding: 15px 17px; background: #f7f9fd; }.editor-meta-title { display: flex; align-items: center; gap: 7px; margin-bottom: 12px; color: #304968; font-size: 14px; font-weight: 700; }.editor-meta-title .el-icon { color: #4779e9; }.editor-workspace { overflow: hidden; background: #fff; }.editor-workspace-head { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-bottom: 1px solid #e7edf6; background: #fbfcff; }.editor-workspace-head > div { display: flex; align-items: center; gap: 10px; }.editor-workspace-head strong { color: #243b5b; }.editor-workspace-head span { color: #94a0b3; font-size: 12px; }.markdown-input :deep(textarea) { min-height: 405px !important; padding: 18px 20px; color: #d8e6ff; font: 13px/1.75 Consolas, 'Microsoft YaHei Mono', monospace; background: #101a2d; border: 0; border-radius: 0; }.markdown-preview { height: 405px; padding: 18px 28px; overflow: auto; color: #3d506e; line-height: 1.75; background: #fff; }.markdown-preview h1 { margin: 0 0 18px; padding-bottom: 10px; color: #142b4b; font-size: 23px; border-bottom: 1px solid #e5ecf5; }.markdown-preview h2 { margin: 21px 0 8px; color: #254f8d; font-size: 17px; }.markdown-preview p { min-height: 12px; margin: 5px 0; white-space: pre-wrap; }.markdown-preview blockquote { margin: 12px 0; padding: 8px 13px; color: #5e7190; background: #f3f7fd; border-left: 3px solid #6084e8; }.markdown-preview pre { padding: 10px; color: #d7e6ff; background: #101a2d; border-radius: 5px; }.retrieval-setting { display: flex; align-items: center; justify-content: space-between; padding: 14px 17px; background: #fbfdfb; }.retrieval-setting div { display: grid; gap: 4px; }.retrieval-setting strong { color: #25405f; }.retrieval-setting span { color: #8391a7; font-size: 12px; }.editor-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; }.editor-footer > span { color: #8896aa; font-size: 12px; } @media(max-width: 760px) { .knowledge-actions, .knowledge-head { align-items: stretch; flex-direction: column; }.document-form-grid { grid-template-columns: 1fr; }.editor-source { display: none; }.editor-dialog-head p { display: none; }.editor-workspace-head { align-items: flex-start; gap: 10px; flex-direction: column; } }
</style>
