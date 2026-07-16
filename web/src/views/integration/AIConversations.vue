<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Delete, Search, StarFilled } from '@element-plus/icons-vue'
import { deleteAIConversation, queryAIConversations, saveAIConversation } from '../../api/integration'
import './ai.css'

const router = useRouter()
const list = ref([]), keyword = ref(''), loading = ref(false)
async function load() { loading.value = true; try { list.value = (await queryAIConversations({ keyword: keyword.value })) || [] } finally { loading.value = false } }
function open(row) { router.push({ path: '/integration/ai/chat', query: { conversationId: row.id } }) }
async function pin(row) { await saveAIConversation({ id: row.id, modelId: row.modelId, title: row.title, pinned: !row.pinned }); await load(); ElMessage.success(row.pinned ? '已取消置顶' : '已置顶') }
async function remove(row) { await ElMessageBox.confirm(`确定删除会话“${row.title}”吗？`, '删除会话', { type: 'warning' }); await deleteAIConversation(row.id); await load(); ElMessage.success('会话已删除') }
function formatTime(value) { return value ? new Date(value).toLocaleString() : '-' }
onMounted(load)
</script>

<template>
  <div class="ai-page">
    <section class="ai-hero"><div><div class="ai-kicker">CONVERSATION MEMORY</div><h1>会话管理</h1><p>集中查看多轮对话、模型来源和最近活动，随时切换回历史上下文。</p></div><el-button type="primary" :icon="ChatLineRound" @click="$router.push('/integration/ai/chat')">开始新会话</el-button></section>
    <section class="ai-panel">
      <div class="conversation-toolbar"><el-input v-model="keyword" clearable placeholder="搜索会话标题" :prefix-icon="Search" @keyup.enter="load"/><el-button type="primary" @click="load">搜索</el-button></div>
      <el-table v-loading="loading" :data="list" @row-dblclick="open">
        <el-table-column label="会话" min-width="300"><template #default="{ row }"><div class="title-cell"><el-icon color="#346bd8"><StarFilled v-if="row.pinned"/><ChatLineRound v-else/></el-icon><span><strong>{{ row.title }}</strong><small>由 {{ row.username || '当前用户' }} 创建</small></span></div></template></el-table-column>
        <el-table-column prop="modelName" label="模型" min-width="180"/>
        <el-table-column prop="messageCount" label="消息数" width="110"/>
        <el-table-column label="最近活动" min-width="190"><template #default="{ row }">{{ formatTime(row.lastMessageAt || row.updateTime) }}</template></el-table-column>
        <el-table-column label="状态" width="100"><template #default><el-tag type="success" effect="plain">可继续</el-tag></template></el-table-column>
        <el-table-column label="操作" width="200"><template #default="{ row }"><el-button link type="primary" @click="open(row)">继续对话</el-button><el-button link @click="pin(row)">{{ row.pinned ? '取消置顶' : '置顶' }}</el-button><el-button link type="danger" :icon="Delete" @click="remove(row)">删除</el-button></template></el-table-column>
      </el-table>
    </section>
  </div>
</template>
<style scoped>.conversation-toolbar { display: flex; gap: 10px; width: min(520px, 100%); padding: 16px 18px; }.title-cell { display: flex; align-items: center; gap: 12px; }.title-cell span, .title-cell strong, .title-cell small { display: block; }.title-cell small { margin-top: 5px; color: #8997aa; }</style>
