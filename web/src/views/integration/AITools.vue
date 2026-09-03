<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Coin, Connection, DataAnalysis, Monitor, Operation } from '@element-plus/icons-vue'
import { executeAITool, queryAITools, updateAITool } from '../../api/integration'
import { it, integrationCategoryKind, integrationCategoryLabel } from '../../utils/integration-i18n'
import './ai.css'

const tools = ref([]), loading = ref(false), testVisible = ref(false), testing = ref(false), activeTool = ref(null)
const args = reactive({ datasourceId: undefined, query: 'up', keyword: '', clusterId: undefined })
const grouped = computed(() => Object.entries(tools.value.reduce((map, item) => { (map[item.category] ||= []).push(item); return map }, {})))
const categoryIcon = (name) => ({ cost: Coin, monitor: DataAnalysis, grafana: Monitor, kubernetes: Operation, integration: Connection }[integrationCategoryKind(name)] || Connection)
async function load() { loading.value = true; try { tools.value = (await queryAITools()) || [] } finally { loading.value = false } }
async function toggle(row) { await updateAITool({ toolKey: row.toolKey, enabled: row.enabled, requireConfirmation: row.requireConfirmation }); ElMessage.success(it('toolUpdated')) }
function openTest(row) { activeTool.value = row; Object.assign(args, { datasourceId: undefined, query: 'up', keyword: '', clusterId: undefined }); testVisible.value = true }
async function execute() {
  testing.value = true
  try {
    const payload = {}
    const properties = activeTool.value?.parameters?.properties || {}
    Object.keys(properties).forEach((key) => { if (args[key] !== undefined && args[key] !== '') payload[key] = args[key] })
    const result = await executeAITool({ toolKey: activeTool.value.toolKey, arguments: payload })
    ElMessageBox.alert(`<pre>${escapeHtml(JSON.stringify(result, null, 2))}</pre>`, it('toolResult'), { dangerouslyUseHTMLString: true, customClass: 'ai-tool-result' })
  } finally { testing.value = false }
}
function escapeHtml(value) { return value.replace(/[&<>]/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[ch]) }
onMounted(load)
</script>

<template>
  <div class="ai-page">
    <section class="ai-hero"><div><div class="ai-kicker">OPERATION TOOL REGISTRY</div><h1>{{ it('toolRegistry') }}</h1><p>{{ it('toolRegistryDesc') }}</p></div><div class="tool-summary"><strong>{{ tools.filter((item) => item.enabled).length }}</strong><span>{{ it('enabledCount', { total: tools.length }) }}</span></div></section>
    <div v-loading="loading" class="tool-groups">
      <section v-for="[category, items] in grouped" :key="category" class="ai-panel tool-section">
        <div class="ai-panel-head"><h2>{{ integrationCategoryLabel(category) }}</h2><span class="ai-muted">{{ it('capabilityCount', { count: items.length }) }}</span></div>
        <div class="tool-grid"><article v-for="item in items" :key="item.toolKey" class="tool-card"><div class="tool-icon"><el-icon><component :is="categoryIcon(category)"/></el-icon></div><div class="tool-copy"><header><strong>{{ item.name }}</strong><el-tag size="small" :type="item.permission === 'write' ? 'warning' : 'success'" effect="plain">{{ item.permission === 'write' ? it('write') : it('readOnly') }}</el-tag></header><p>{{ item.description }}</p><code>{{ item.toolKey }}</code></div><div class="tool-controls"><el-button v-if="item.permission === 'read'" link type="primary" @click="openTest(item)">{{ it('debug') }}</el-button><el-switch v-model="item.enabled" @change="toggle(item)"/></div></article></div>
      </section>
    </div>
    <el-dialog v-model="testVisible" :title="it('debugTool', { name: activeTool?.name || '' })" width="620px"><el-form label-position="top"><el-form-item v-for="(schema, key) in activeTool?.parameters?.properties || {}" :key="key" :label="schema.description || key"><el-input-number v-if="schema.type === 'integer'" v-model="args[key]" :min="0" style="width:100%"/><el-input v-else v-model="args[key]" :type="key === 'query' ? 'textarea' : 'text'" :rows="4"/></el-form-item></el-form><template #footer><el-button @click="testVisible = false">{{ it('cancel') }}</el-button><el-button type="primary" :loading="testing" @click="execute">{{ it('executeReadOnly') }}</el-button></template></el-dialog>
  </div>
</template>
<style scoped>.tool-summary{display:flex;align-items:baseline;gap:8px}.tool-summary strong{color:#2762d4;font-size:30px}.tool-summary span{color:#7d8da5}.tool-groups{display:grid;gap:16px}.tool-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:1px;background:#e4eaf3}.tool-card{display:grid;grid-template-columns:44px 1fr auto;gap:13px;min-height:120px;padding:18px;background:#fff}.tool-icon{display:grid;place-items:center;width:42px;height:42px;color:#2f69d8;font-size:22px;background:#eaf1ff;border-radius:6px}.tool-copy header{display:flex;align-items:center;gap:9px}.tool-copy p{margin:9px 0;color:#6f809b;line-height:1.55}.tool-copy code{color:#46617f;font-size:12px}.tool-controls{display:flex;align-items:center;gap:8px}@media(max-width:900px){.tool-grid{grid-template-columns:1fr}}</style>
<style>.ai-tool-result pre{max-height:55vh;overflow:auto;color:#d7e3f8;font:13px/1.6 Consolas,monospace;text-align:left;background:#101a2d;padding:16px;border-radius:6px}</style>
