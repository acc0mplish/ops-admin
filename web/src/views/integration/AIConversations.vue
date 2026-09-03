<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Delete, Search, StarFilled } from '@element-plus/icons-vue'
import { deleteAIConversation, queryAIConversations, saveAIConversation } from '../../api/integration'
import { it } from '../../utils/integration-i18n'
import './ai.css'

const router = useRouter()
const list = ref([]), keyword = ref(''), loading = ref(false)
async function load() { loading.value = true; try { list.value = (await queryAIConversations({ keyword: keyword.value })) || [] } finally { loading.value = false } }
function open(row) { router.push({ path: '/integration/ai/chat', query: { conversationId: row.id } }) }
async function pin(row) { await saveAIConversation({ id: row.id, modelId: row.modelId, title: row.title, pinned: !row.pinned }); await load(); ElMessage.success(row.pinned ? it('conversationUnpinned') : it('conversationPinned')) }
async function remove(row) { await ElMessageBox.confirm(it('deleteConversationConfirm', { title: row.title }), it('deleteConversation'), { type: 'warning' }); await deleteAIConversation(row.id); await load(); ElMessage.success(it('conversationDeleted')) }
function formatTime(value) { return value ? new Date(value).toLocaleString() : '-' }
onMounted(load)
</script>

<template>
  <div class="ai-page">
    <section class="ai-hero"><div><div class="ai-kicker">{{ it('conversationManagement') }}</div><h1>{{ it('conversationManagement') }}</h1><p>{{ it('conversationManagementDesc') }}</p></div><el-button type="primary" :icon="ChatLineRound" @click="$router.push('/integration/ai/chat')">{{ it('newConversation') }}</el-button></section>
    <section class="ai-panel">
      <div class="conversation-toolbar"><el-input v-model="keyword" clearable :placeholder="it('searchConversationTitle')" :prefix-icon="Search" @keyup.enter="load"/><el-button type="primary" @click="load">{{ it('search') }}</el-button></div>
      <el-table v-loading="loading" :data="list" @row-dblclick="open">
        <el-table-column :label="it('conversation')" min-width="300"><template #default="{ row }"><div class="title-cell"><el-icon color="#346bd8"><StarFilled v-if="row.pinned"/><ChatLineRound v-else/></el-icon><span><strong>{{ row.title }}</strong><small>{{ it('createdBy', { name: row.username || it('currentUser') }) }}</small></span></div></template></el-table-column>
        <el-table-column prop="modelName" :label="it('model')" min-width="180"/>
        <el-table-column prop="messageCount" :label="it('messageCount')" width="110"/>
        <el-table-column :label="it('recentActivity')" min-width="190"><template #default="{ row }">{{ formatTime(row.lastMessageAt || row.updateTime) }}</template></el-table-column>
        <el-table-column :label="it('status')" width="100"><template #default><el-tag type="success" effect="plain">{{ it('resumable') }}</el-tag></template></el-table-column>
        <el-table-column :label="it('actions')" width="200"><template #default="{ row }"><el-button link type="primary" @click="open(row)">{{ it('continueConversation') }}</el-button><el-button link @click="pin(row)">{{ row.pinned ? it('unpin') : it('pin') }}</el-button><el-button link type="danger" :icon="Delete" @click="remove(row)">{{ it('delete') }}</el-button></template></el-table-column>
      </el-table>
    </section>
  </div>
</template>
<style scoped>.conversation-toolbar{display:flex;gap:10px;width:min(520px,100%);padding:16px 18px}.title-cell{display:flex;align-items:center;gap:12px}.title-cell span,.title-cell strong,.title-cell small{display:block}.title-cell small{margin-top:5px;color:#8997aa}</style>
