<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Delete, Edit, Key, Plus, Setting } from '@element-plus/icons-vue'
import { deleteAIModel, queryAIModels, saveAIModel, testAIModel } from '../../api/integration'
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
async function test() { testing.value = true; try { const data = await testAIModel(form); ElMessage.success(`连接成功，耗时 ${data.latencyMs} ms`) } finally { testing.value = false } }
async function submit() {
  if (!form.name.trim() || !form.baseUrl.trim() || !form.model.trim()) return ElMessage.warning('请填写模型名称、API 地址和模型标识')
  saving.value = true
  try { await saveAIModel(form); dialogVisible.value = false; await load(); ElMessage.success('模型配置已保存') } finally { saving.value = false }
}
async function remove(row) { await ElMessageBox.confirm(`确定删除模型“${row.name}”吗？`, '删除模型', { type: 'warning' }); await deleteAIModel(row.id); await load(); ElMessage.success('模型已删除') }
onMounted(load)
</script>

<template>
  <div class="ai-page">
    <section class="ai-hero">
      <div><div class="ai-kicker">OPENAI COMPATIBLE MODELS</div><h1>模型管理</h1><p>统一维护 OpenAI 兼容接口、模型参数和默认模型，密钥不会返回到浏览器。</p></div>
      <el-button type="primary" :icon="Plus" @click="createModel">新增模型</el-button>
    </section>
    <section class="ai-panel">
      <div class="ai-panel-head"><h2>模型配置</h2><span class="ai-muted">已配置 {{ models.length }} 个模型</span></div>
      <el-table v-loading="loading" :data="models" empty-text="尚未配置模型" style="width: 100%">
        <el-table-column label="模型名称" min-width="180"><template #default="{ row }"><div class="model-name"><i class="ai-status-dot" :class="{ off: row.status !== 1 }"></i><strong>{{ row.name }}</strong><el-tag v-if="row.isDefault" size="small" type="primary">默认</el-tag></div><small>{{ row.description || 'OpenAI 兼容模型' }}</small></template></el-table-column>
        <el-table-column prop="model" label="模型标识" min-width="160"><template #default="{ row }"><code>{{ row.model }}</code></template></el-table-column>
        <el-table-column prop="baseUrl" label="API 地址" min-width="240" show-overflow-tooltip/>
        <el-table-column label="凭据" width="130"><template #default="{ row }"><el-tag :type="row.hasApiKey ? 'success' : 'info'" effect="plain">{{ row.hasApiKey ? row.apiKeyMasked : '未配置' }}</el-tag></template></el-table-column>
        <el-table-column label="参数" width="170"><template #default="{ row }"><span class="ai-muted">T {{ row.temperature }} · {{ row.maxTokens }} tokens</span></template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="150"><template #default="{ row }"><div class="ai-table-actions"><el-button link type="primary" :icon="Edit" @click="editModel(row)">编辑</el-button><el-button link type="danger" :icon="Delete" @click="remove(row)">删除</el-button></div></template></el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" width="min(1080px, 88vw)" class="ai-model-dialog" destroy-on-close>
      <template #header>
        <div class="model-dialog-title">
          <span class="model-dialog-icon"><el-icon><Setting /></el-icon></span>
          <div>
            <h2>{{ form.id ? '编辑模型' : '新增模型' }}</h2>
            <p>配置 OpenAI 兼容接口、访问凭据和模型生成参数。</p>
          </div>
        </div>
      </template>

      <el-form class="model-form" label-position="top">
        <section class="model-section">
          <div class="model-section-head">
            <div><span>01</span><h3>基础信息</h3></div>
            <p>用于在智能对话和工具调用中识别该模型。</p>
          </div>
          <div class="form-grid basic-grid">
            <el-form-item label="模型名称" required>
              <el-input v-model="form.name" placeholder="例如：生产运维助手" clearable />
            </el-form-item>
            <el-form-item label="模型标识" required>
              <el-input v-model="form.model" placeholder="例如：gpt-4.1-mini" clearable />
            </el-form-item>
            <el-form-item label="模型说明">
              <el-input v-model="form.description" placeholder="说明模型用途或适用场景" clearable />
            </el-form-item>
          </div>
        </section>

        <section class="model-section">
          <div class="model-section-head">
            <div><span>02</span><h3>连接与凭据</h3></div>
            <p>填写兼容 OpenAI Chat Completions 的 API 根地址。</p>
          </div>
          <div class="form-grid connection-grid">
            <el-form-item label="OpenAI 兼容 API 地址" required>
              <el-input v-model="form.baseUrl" placeholder="https://api.example.com/v1" clearable>
                <template #prepend>HTTPS</template>
              </el-input>
              <div class="field-help">系统将自动请求 <code>/chat/completions</code> 接口。</div>
            </el-form-item>
            <el-form-item label="API Key">
              <el-input v-model="form.apiKey" type="password" show-password :prefix-icon="Key" :placeholder="form.id ? '留空保持原密钥不变' : 'sk-...'" />
              <div class="field-help">密钥仅保存于服务端，页面不会回显完整内容。</div>
            </el-form-item>
          </div>
        </section>

        <section class="model-section">
          <div class="model-section-head">
            <div><span>03</span><h3>生成参数</h3></div>
            <p>控制回答随机性、输出上限和单次请求等待时间。</p>
          </div>
          <div class="parameter-grid">
            <label class="parameter-field">
              <span>Temperature</span>
              <small>数值越低，回答越稳定</small>
              <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" :precision="1" controls-position="right" />
            </label>
            <label class="parameter-field">
              <span>最大 Tokens</span>
              <small>限制单次回答的最大长度</small>
              <el-input-number v-model="form.maxTokens" :min="1" :max="393216" :step="128" controls-position="right" />
            </label>
            <label class="parameter-field">
              <span>请求超时</span>
              <small>范围 5 至 600 秒</small>
              <el-input-number v-model="form.timeoutSeconds" :min="5" :max="600" controls-position="right" />
            </label>
          </div>
          <el-form-item class="prompt-field" label="附加系统指令">
            <el-input v-model="form.systemPrompt" type="textarea" :rows="4" resize="none" placeholder="例如：所有变更建议必须先说明影响范围、风险和回滚方案。" />
            <div class="field-help">该指令会追加到平台内置的运维安全提示之后。</div>
          </el-form-item>
        </section>

        <div class="model-switches">
          <div>
            <strong>启用模型</strong>
            <span>停用后不可在智能对话中选择</span>
            <el-switch v-model="form.status" :active-value="1" :inactive-value="2" />
          </div>
          <div>
            <strong>设为默认模型</strong>
            <span>新会话将优先使用该模型</span>
            <el-switch v-model="form.isDefault" />
          </div>
        </div>
      </el-form>

      <template #footer>
        <div class="model-dialog-footer">
          <el-button :icon="Connection" :loading="testing" @click="test">测试连接</el-button>
          <div><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存模型</el-button></div>
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
