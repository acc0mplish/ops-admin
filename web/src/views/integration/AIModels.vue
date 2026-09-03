<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Delete, Edit, Key, Plus, Setting } from '@element-plus/icons-vue'
import { deleteAIModel, queryAIModels, saveAIModel, testAIModel } from '../../api/integration'
import { amt } from '../../utils/ai-model-i18n'
import './ai.css'

const loading = ref(false)
const models = ref([])
const dialogVisible = ref(false)
const saving = ref(false)
const testing = ref(false)
const form = reactive({ id: undefined, name: '', provider: 'openai_compatible', baseUrl: '', apiKey: '', model: '', systemPrompt: '', temperature: 0.2, maxTokens: 2048, timeoutSeconds: 60, isDefault: false, status: 1, description: '' })

function reset() { Object.assign(form, { id: undefined, name: '', provider: 'openai_compatible', baseUrl: '', apiKey: '', model: '', systemPrompt: '', temperature: 0.2, maxTokens: 2048, timeoutSeconds: 60, isDefault: false, status: 1, description: '' }) }
async function load() { loading.value = true; try { models.value = (await queryAIModels()) || [] } finally { loading.value = false } }
function createModel() { reset(); dialogVisible.value = true }
function editModel(row) { reset(); Object.assign(form, row, { apiKey: '' }); dialogVisible.value = true }
async function test() { testing.value = true; try { const data = await testAIModel(form); ElMessage.success(amt('connectionSuccess', { latency: data.latencyMs })) } finally { testing.value = false } }
async function submit() {
  if (!form.name.trim() || !form.baseUrl.trim() || !form.model.trim()) return ElMessage.warning(amt('requiredFields'))
  saving.value = true
  try { await saveAIModel(form); dialogVisible.value = false; await load(); ElMessage.success(amt('modelSaved')) } finally { saving.value = false }
}
async function remove(row) {
  await ElMessageBox.confirm(amt('deleteModelConfirm', { name: row.name }), amt('deleteModelTitle'), { type: 'warning', confirmButtonText: amt('delete'), cancelButtonText: amt('cancel') })
  await deleteAIModel(row.id)
  await load()
  ElMessage.success(amt('modelDeleted'))
}
onMounted(load)
</script>

<template>
  <div class="ai-page">
    <section class="ai-hero">
      <div><div class="ai-kicker">{{ amt('pageEyebrow') }}</div><h1>{{ amt('pageTitle') }}</h1><p>{{ amt('pageDescription') }}</p></div>
      <el-button type="primary" :icon="Plus" @click="createModel">{{ amt('addModel') }}</el-button>
    </section>
    <section class="ai-panel">
      <div class="ai-panel-head"><h2>{{ amt('modelConfig') }}</h2><span class="ai-muted">{{ amt('configuredCount', { count: models.length }) }}</span></div>
      <el-table v-loading="loading" :data="models" :empty-text="amt('noModels')" style="width: 100%">
        <el-table-column :label="amt('modelName')" min-width="180"><template #default="{ row }"><div class="model-name"><i class="ai-status-dot" :class="{ off: row.status !== 1 }"></i><strong>{{ row.name }}</strong><el-tag v-if="row.isDefault" size="small" type="primary">{{ amt('default') }}</el-tag></div><small>{{ row.description || amt('compatibleModel') }}</small></template></el-table-column>
        <el-table-column prop="model" :label="amt('modelId')" min-width="160"><template #default="{ row }"><code>{{ row.model }}</code></template></el-table-column>
        <el-table-column prop="baseUrl" :label="amt('apiAddress')" min-width="240" show-overflow-tooltip/>
        <el-table-column :label="amt('credential')" width="130"><template #default="{ row }"><el-tag :type="row.hasApiKey ? 'success' : 'info'" effect="plain">{{ row.hasApiKey ? row.apiKeyMasked : amt('notConfigured') }}</el-tag></template></el-table-column>
        <el-table-column :label="amt('parameters')" width="170"><template #default="{ row }"><span class="ai-muted">T {{ row.temperature }} · {{ row.maxTokens }} {{ amt('tokens') }}</span></template></el-table-column>
        <el-table-column :label="amt('status')" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? amt('enabled') : amt('disabled') }}</el-tag></template></el-table-column>
        <el-table-column :label="amt('actions')" width="150"><template #default="{ row }"><div class="ai-table-actions"><el-button link type="primary" :icon="Edit" @click="editModel(row)">{{ amt('edit') }}</el-button><el-button link type="danger" :icon="Delete" @click="remove(row)">{{ amt('delete') }}</el-button></div></template></el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" width="min(1080px, 88vw)" class="ai-model-dialog" destroy-on-close>
      <template #header>
        <div class="model-dialog-title">
          <span class="model-dialog-icon"><el-icon><Setting /></el-icon></span>
          <div>
            <h2>{{ form.id ? amt('editModel') : amt('newModel') }}</h2>
            <p>{{ amt('dialogDescription') }}</p>
          </div>
        </div>
      </template>

      <el-form class="model-form" label-position="top">
        <section class="model-section">
          <div class="model-section-head">
            <div><span>01</span><h3>{{ amt('basicInfo') }}</h3></div>
            <p>{{ amt('basicInfoDescription') }}</p>
          </div>
          <div class="form-grid basic-grid">
            <el-form-item :label="amt('modelName')" required>
              <el-input v-model="form.name" :placeholder="amt('modelNamePlaceholder')" clearable />
            </el-form-item>
            <el-form-item :label="amt('modelId')" required>
              <el-input v-model="form.model" :placeholder="amt('modelIdPlaceholder')" clearable />
            </el-form-item>
            <el-form-item :label="amt('modelDescription')">
              <el-input v-model="form.description" :placeholder="amt('modelDescriptionPlaceholder')" clearable />
            </el-form-item>
          </div>
        </section>

        <section class="model-section">
          <div class="model-section-head">
            <div><span>02</span><h3>{{ amt('connectionCredential') }}</h3></div>
            <p>{{ amt('connectionDescription') }}</p>
          </div>
          <div class="form-grid connection-grid">
            <el-form-item :label="amt('compatibleApiAddress')" required>
              <el-input v-model="form.baseUrl" placeholder="https://api.example.com/v1" clearable>
                <template #prepend>HTTPS</template>
              </el-input>
              <div class="field-help">{{ amt('requestEndpointHint') }}</div>
            </el-form-item>
            <el-form-item :label="amt('apiKey')">
              <el-input v-model="form.apiKey" type="password" show-password :prefix-icon="Key" :placeholder="form.id ? amt('keepExistingKey') : 'sk-...'" />
              <div class="field-help">{{ amt('keyStoredServerSide') }}</div>
            </el-form-item>
          </div>
        </section>

        <section class="model-section">
          <div class="model-section-head">
            <div><span>03</span><h3>{{ amt('generationParameters') }}</h3></div>
            <p>{{ amt('generationDescription') }}</p>
          </div>
          <div class="parameter-grid">
            <label class="parameter-field">
              <span>{{ amt('temperature') }}</span>
              <small>{{ amt('temperatureHint') }}</small>
              <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" :precision="1" controls-position="right" />
            </label>
            <label class="parameter-field">
              <span>{{ amt('maxTokens') }}</span>
              <small>{{ amt('maxTokensHint') }}</small>
              <el-input-number v-model="form.maxTokens" :min="1" :max="393216" :step="128" controls-position="right" />
            </label>
            <label class="parameter-field">
              <span>{{ amt('requestTimeout') }}</span>
              <small>{{ amt('requestTimeoutHint') }}</small>
              <el-input-number v-model="form.timeoutSeconds" :min="5" :max="600" controls-position="right" />
            </label>
          </div>
          <el-form-item class="prompt-field" :label="amt('additionalSystemInstruction')">
            <el-input v-model="form.systemPrompt" type="textarea" :rows="4" resize="none" :placeholder="amt('systemPromptPlaceholder')" />
            <div class="field-help">{{ amt('systemInstructionHint') }}</div>
          </el-form-item>
        </section>

        <div class="model-switches">
          <div>
            <strong>{{ amt('enableModel') }}</strong>
            <span>{{ amt('enableModelHint') }}</span>
            <el-switch v-model="form.status" :active-value="1" :inactive-value="2" />
          </div>
          <div>
            <strong>{{ amt('setDefaultModel') }}</strong>
            <span>{{ amt('defaultModelHint') }}</span>
            <el-switch v-model="form.isDefault" />
          </div>
        </div>
      </el-form>

      <template #footer>
        <div class="model-dialog-footer">
          <el-button :icon="Connection" :loading="testing" @click="test">{{ amt('testConnection') }}</el-button>
          <div><el-button @click="dialogVisible = false">{{ amt('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ amt('saveModel') }}</el-button></div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.model-name { display: flex; align-items: center; gap: 8px; }
.model-name + small { display: block; margin: 5px 0 0 15px; color: #8593a8; }
.ai-status-dot.off { background: #aab4c3; box-shadow: none; }
code { color: #235ab6; }
.model-dialog-title { display: flex; align-items: center; gap: 12px; }
.model-dialog-title h2 { margin: 0 0 4px; color: #10213f; font-size: 20px; }
.model-dialog-title p { margin: 0; color: #7b8ba4; font-size: 13px; }
.model-dialog-icon { display: grid; place-items: center; width: 40px; height: 40px; color: #fff; background: #356fe5; border-radius: 7px; font-size: 20px; }
.model-form { max-height: min(68vh, 680px); padding: 0 2px; overflow-y: auto; }
.model-section { padding: 20px 2px 18px; border-bottom: 1px solid #e6edf7; }
.model-section:first-child { padding-top: 4px; }
.model-section-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 16px; }
.model-section-head > div { display: flex; align-items: center; gap: 9px; }
.model-section-head span { color: #356fe5; font: 700 12px Consolas, monospace; }
.model-section-head h3 { margin: 0; color: #172a49; font-size: 16px; }
.model-section-head p { margin: 0; color: #8190a7; font-size: 13px; }
.form-grid { display: grid; gap: 16px; }
.basic-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.connection-grid { grid-template-columns: minmax(0, 1.35fr) minmax(280px, .65fr); }
.model-form :deep(.el-form-item) { margin-bottom: 0; }
.model-form :deep(.el-form-item__label) { padding-bottom: 7px; color: #40516e; font-weight: 600; }
.field-help { margin-top: 6px; color: #8997ac; font-size: 12px; line-height: 1.4; }
.parameter-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.parameter-field { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 4px 12px; padding: 14px 15px; background: #f6f8fc; border: 1px solid #e2e9f4; border-radius: 6px; }
.parameter-field > span { color: #304461; font-weight: 600; }
.parameter-field small { grid-column: 1; color: #8a98ad; }
.parameter-field .el-input-number { grid-column: 2; grid-row: 1 / span 2; width: 142px; }
.prompt-field { margin-top: 16px !important; }
.model-switches { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; padding-top: 16px; }
.model-switches > div { display: grid; grid-template-columns: 1fr auto; gap: 4px 16px; padding: 14px 16px; border: 1px solid #e2e9f4; border-radius: 6px; }
.model-switches strong { color: #243955; font-size: 14px; }
.model-switches span { color: #8795aa; font-size: 12px; }
.model-switches .el-switch { grid-column: 2; grid-row: 1 / span 2; }
.model-dialog-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; }
@media(max-width: 900px) {
  .basic-grid, .connection-grid, .parameter-grid, .model-switches { grid-template-columns: 1fr; }
  .model-section-head { align-items: flex-start; flex-direction: column; gap: 5px; }
}
</style>
