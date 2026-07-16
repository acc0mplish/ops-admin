<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Delete, Plus, Promotion, Setting, Star, StarFilled } from '@element-plus/icons-vue'
import {
  confirmAIAction,
  deleteAIConversation,
  queryAIConversationDetail,
  queryAIConversations,
  queryAIModels,
  rejectAIAction,
  saveAIConversation,
  sendAIChat
} from '../../api/integration'
import './ai.css'

const models = ref([])
const conversations = ref([])
const currentId = ref(0)
const currentModelId = ref(0)
const messages = ref([])
const actions = ref([])
const input = ref('')
const sending = ref(false)
const scrollRef = ref()
const route = useRoute()

const activeConversation = computed(() => conversations.value.find((item) => item.id === currentId.value))
const pendingActions = computed(() => actions.value.filter((item) => item.status === 'pending'))
const suggestions = ['检查当前 K8s 集群健康状态', '查询最近主机 CPU 使用率', '列出可用监控大屏', '分析 Pod 频繁重启的排障步骤']

async function loadModels() {
  models.value = (await queryAIModels()) || []
  const enabled = models.value.filter((item) => item.status === 1)
  if (!currentModelId.value) currentModelId.value = enabled.find((item) => item.isDefault)?.id || enabled[0]?.id || 0
}

async function loadConversations(preferredId = currentId.value) {
  conversations.value = (await queryAIConversations()) || []
  if (preferredId && conversations.value.some((item) => item.id === preferredId)) currentId.value = preferredId
}

async function openConversation(id) {
  currentId.value = id
  const data = await queryAIConversationDetail(id)
  messages.value = data?.messages || []
  actions.value = data?.actions || []
  currentModelId.value = data?.conversation?.modelId || currentModelId.value
  await scrollToBottom()
}

function newConversation() {
  currentId.value = 0
  messages.value = []
  actions.value = []
  input.value = ''
}

async function send(content = input.value) {
  const text = String(content || '').trim()
  if (!text || sending.value) return
  if (!currentModelId.value) {
    ElMessage.warning('请先在模型管理中配置并启用一个模型')
    return
  }
  messages.value.push({ id: `local-${Date.now()}`, role: 'user', content: text, createTime: new Date().toISOString() })
  input.value = ''
  sending.value = true
  await scrollToBottom()
  try {
    const data = await sendAIChat({ conversationId: currentId.value, modelId: currentModelId.value, content: text })
    currentId.value = data.conversationId
    messages.value.push(data.message)
    actions.value.push(...(data.actions || []))
    await loadConversations(currentId.value)
  } finally {
    sending.value = false
    await scrollToBottom()
  }
}

async function scrollToBottom() {
  await nextTick()
  const el = scrollRef.value
  if (el) el.scrollTop = el.scrollHeight
}

async function togglePin() {
  if (!activeConversation.value) return
  await saveAIConversation({ id: activeConversation.value.id, modelId: currentModelId.value, title: activeConversation.value.title, pinned: !activeConversation.value.pinned })
  await loadConversations(currentId.value)
}

async function removeConversation() {
  if (!currentId.value) return
  await ElMessageBox.confirm('删除后将同时清理该会话的消息与待确认操作，是否继续？', '删除会话', { type: 'warning' })
  await deleteAIConversation(currentId.value)
  newConversation()
  await loadConversations()
  ElMessage.success('会话已删除')
}

async function handleAction(action, accepted) {
  if (accepted) {
    await ElMessageBox.confirm('该操作将修改 K8s 资源，请确认目标集群与参数无误。', '确认执行变更', { type: 'warning', confirmButtonText: '确认执行' })
    await confirmAIAction(action.id)
    ElMessage.success('操作执行完成')
  } else {
    await rejectAIAction(action.id)
    ElMessage.success('已拒绝执行')
  }
  await openConversation(currentId.value)
}

function keydown(event) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    send()
  }
}

onMounted(async () => {
  await Promise.all([loadModels(), loadConversations()])
  const requestedId = Number(route.query.conversationId || 0)
  if (requestedId) await openConversation(requestedId)
})
</script>

<template>
  <div class="ai-page chat-page">
    <section class="ai-hero">
      <div>
        <div class="ai-kicker">AI OPERATIONS COPILOT</div>
        <h1>智能对话</h1>
        <p>用自然语言查询监控与 K8s 资源；所有变更操作都需要人工确认。</p>
      </div>
      <div class="hero-actions">
        <el-select v-model="currentModelId" placeholder="选择模型" style="width: 220px">
          <el-option v-for="item in models.filter((row) => row.status === 1)" :key="item.id" :label="item.name" :value="item.id">
            <span>{{ item.name }}</span><span class="model-code">{{ item.model }}</span>
          </el-option>
        </el-select>
        <el-button :icon="Plus" type="primary" @click="newConversation">新建会话</el-button>
      </div>
    </section>

    <section class="chat-shell ai-panel">
      <aside class="conversation-rail">
        <div class="rail-title"><strong>最近会话</strong><span>{{ conversations.length }}</span></div>
        <div class="conversation-list">
          <button v-for="item in conversations" :key="item.id" class="conversation-item" :class="{ active: item.id === currentId }" @click="openConversation(item.id)">
            <el-icon><StarFilled v-if="item.pinned"/><ChatLineRound v-else/></el-icon>
            <span><strong>{{ item.title }}</strong><small>{{ item.modelName || '未配置模型' }} · {{ item.messageCount }} 条</small></span>
          </button>
        </div>
        <div class="rail-footer">
          <el-button text :icon="Setting" @click="$router.push('/integration/ai/models')">模型管理</el-button>
        </div>
      </aside>

      <main class="chat-main">
        <header class="chat-head">
          <div><strong>{{ activeConversation?.title || '新会话' }}</strong><span><i class="ai-status-dot"></i>工具调用已启用</span></div>
          <div v-if="currentId">
            <el-button circle :icon="activeConversation?.pinned ? StarFilled : Star" title="置顶会话" @click="togglePin"/>
            <el-button circle :icon="Delete" title="删除会话" @click="removeConversation"/>
          </div>
        </header>

        <div ref="scrollRef" class="message-stage">
          <div v-if="!messages.length" class="welcome-block">
            <div class="assistant-mark">AI</div>
            <h2>今天需要排查什么？</h2>
            <p>我可以读取平台中的监控、Grafana 大屏和 Kubernetes 信息。</p>
            <div class="suggestions">
              <button v-for="item in suggestions" :key="item" @click="send(item)">{{ item }}</button>
            </div>
          </div>
          <template v-else>
            <article v-for="message in messages" :key="message.id" class="message" :class="message.role">
              <div class="message-avatar">{{ message.role === 'user' ? '我' : 'AI' }}</div>
              <div class="message-body"><div class="message-role">{{ message.role === 'user' ? '你' : '运维助手' }}</div><pre>{{ message.content }}</pre></div>
            </article>
          </template>

          <div v-for="action in pendingActions" :key="action.id" class="action-card">
            <div><strong>待确认的变更操作</strong><el-tag type="warning" effect="plain">{{ action.toolKey }}</el-tag></div>
            <pre>{{ action.argumentsJson }}</pre>
            <footer><el-button @click="handleAction(action, false)">拒绝</el-button><el-button type="warning" @click="handleAction(action, true)">确认执行</el-button></footer>
          </div>
          <div v-if="sending" class="thinking"><i></i><i></i><i></i><span>正在分析平台数据</span></div>
        </div>

        <footer class="composer">
          <el-input v-model="input" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" resize="none" placeholder="描述现象、资源范围或期望执行的操作，Enter 发送，Shift + Enter 换行" @keydown="keydown"/>
          <div class="composer-foot"><span>只读工具自动执行，K8s 变更必须二次确认</span><el-button type="primary" :icon="Promotion" :loading="sending" @click="send()">发送</el-button></div>
        </footer>
      </main>
    </section>
  </div>
</template>

<style scoped>
.chat-page { display: flex; flex-direction: column; height: calc(100dvh - 166px); min-height: 0; overflow: hidden; }
.chat-page > .ai-hero { flex: 0 0 auto; min-height: 0; padding: 8px 24px; margin-bottom: 10px; }
.chat-page > .ai-hero .ai-kicker { margin-bottom: 1px; line-height: 15px; }
.chat-page > .ai-hero h1 { margin-bottom: 2px; font-size: 24px; line-height: 29px; }
.chat-page > .ai-hero p { font-size: 13px; line-height: 18px; }
.hero-actions { display: flex; align-items: center; gap: 10px; }
.model-code { float: right; margin-left: 18px; color: #8a98ad; font-size: 12px; }
.chat-shell { display: grid; flex: 1 1 auto; grid-template-columns: 260px minmax(0, 1fr); min-height: 0; overflow: hidden; }
.conversation-rail { display: flex; flex-direction: column; min-width: 0; background: #f7f9fd; border-right: 1px solid #e0e8f4; }
.rail-title { display: flex; justify-content: space-between; padding: 18px; color: #526884; }
.conversation-list { flex: 1; padding: 0 10px; overflow: auto; }
.conversation-item { display: flex; align-items: flex-start; width: 100%; gap: 10px; padding: 12px; margin-bottom: 5px; color: #49607e; text-align: left; background: transparent; border: 0; border-radius: 6px; cursor: pointer; }
.conversation-item:hover, .conversation-item.active { color: #174ca6; background: #e8f0ff; }
.conversation-item span { min-width: 0; }
.conversation-item strong, .conversation-item small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conversation-item strong { margin-bottom: 5px; color: #182b49; }
.conversation-item small { color: #8391a7; }
.rail-footer { padding: 10px; border-top: 1px solid #e0e8f4; }
.chat-main { display: grid; grid-template-rows: 58px minmax(0, 1fr) auto; min-width: 0; min-height: 0; overflow: hidden; }
.chat-head { display: flex; align-items: center; justify-content: space-between; padding: 0 20px; border-bottom: 1px solid #e4ebf5; }
.chat-head strong, .chat-head span { display: block; }
.chat-head span { margin-top: 3px; color: #8290a6; font-size: 12px; }
.message-stage { min-height: 0; padding: 22px 28px; overflow: auto; overscroll-behavior: contain; background: #fbfcfe; scrollbar-gutter: stable; }
.welcome-block { max-width: 760px; margin: 8vh auto 0; text-align: center; }
.assistant-mark { display: grid; place-items: center; width: 52px; height: 52px; margin: auto; color: #fff; font-weight: 800; background: #316bdf; border-radius: 8px; }
.welcome-block h2 { margin: 18px 0 8px; }
.welcome-block p { color: #7889a3; }
.suggestions { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; margin-top: 28px; }
.suggestions button { min-height: 52px; padding: 10px 16px; color: #334b6c; text-align: left; background: #fff; border: 1px solid #dce5f2; border-radius: 6px; cursor: pointer; }
.suggestions button:hover { color: #2463d4; border-color: #7da7f0; }
.message { display: flex; width: min(100%, 1320px); gap: 12px; margin: 0 auto 22px; }
.message.user { flex-direction: row-reverse; }
.message-avatar { display: grid; place-items: center; flex: 0 0 34px; height: 34px; color: #fff; font-size: 12px; font-weight: 700; background: #316bdf; border-radius: 6px; }
.message.user .message-avatar { background: #172a49; }
.message-body { min-width: 0; max-width: min(88%, 1120px); }
.message.assistant .message-body { width: min(88%, 1120px); }
.message.user .message-body { max-width: min(76%, 920px); }
.message-role { margin-bottom: 6px; color: #7788a3; font-size: 12px; }
.message.user .message-role { text-align: right; }
.message pre { width: 100%; max-width: none; box-sizing: border-box; margin: 0; padding: 14px 16px; overflow-wrap: anywhere; color: #263a58; font-family: inherit; line-height: 1.75; white-space: pre-wrap; background: #fff; border: 1px solid #dfe7f2; border-radius: 7px; }
.message.user pre { color: #fff; background: #315fba; border-color: #315fba; }
.action-card { max-width: 760px; padding: 16px; margin: 0 auto 22px; background: #fffaf0; border: 1px solid #f0cd85; border-radius: 7px; }
.action-card > div { display: flex; justify-content: space-between; }
.action-card pre { padding: 10px; overflow: auto; color: #6f5218; background: #fff; border-radius: 4px; }
.action-card footer { text-align: right; }
.thinking { display: flex; align-items: center; justify-content: center; gap: 5px; color: #7586a1; }
.thinking i { width: 7px; height: 7px; background: #4d7fe0; border-radius: 50%; animation: pulse 1s infinite alternate; }
.thinking i:nth-child(2) { animation-delay: .2s; }.thinking i:nth-child(3) { animation-delay: .4s; }.thinking span { margin-left: 6px; }
@keyframes pulse { to { opacity: .25; transform: translateY(-3px); } }
.composer { position: relative; z-index: 2; flex: 0 0 auto; padding: 12px 20px 14px; background: #fff; border-top: 1px solid #e2e9f3; box-shadow: 0 -8px 20px rgba(31, 55, 91, .04); }
.composer-foot { display: flex; align-items: center; justify-content: space-between; margin-top: 8px; color: #8795aa; font-size: 12px; }
@media (max-width: 900px) { .chat-page { height: calc(100dvh - 146px); }.chat-page > .ai-hero { display: none; }.chat-shell { grid-template-columns: 1fr; }.conversation-rail { display: none; }.message-stage { padding: 18px 14px; }.message-body, .message.assistant .message-body { width: auto; max-width: calc(100% - 46px); }.message.user .message-body { max-width: 86%; }.suggestions { grid-template-columns: 1fr; } }
</style>
