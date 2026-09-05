<script setup>
import { computed, nextTick, onActivated, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { ElMessage } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import { getToken } from '../../utils/auth'

const loading = ref(false)
const treeRef = ref()
const terminalBoxRef = ref()
const groupTree = ref([])
const sessions = ref([])
const activeSessionId = ref()
const query = reactive({ keyword: '' })

// xterm and WebSocket instances must stay outside Vue reactivity. Each opened
// host has an independent runtime, so switching a tab never disconnects it.
const runtimes = new Map()
const terminalElements = new Map()
let resizeObserver
let resizeFrame

const activeSession = computed(() => sessions.value.find((item) => item.id === activeSessionId.value))

async function loadTree() {
  loading.value = true
  try {
    const [groupsRes, hostsRes] = await Promise.all([
      queryAssetHostGroupList({ keyword: query.keyword }),
      queryAssetHostList({ pageNum: 1, pageSize: 1000, keyword: query.keyword })
    ])
    groupTree.value = buildTree(groupsRes.list || [], hostsRes.list || [])
  } finally {
    loading.value = false
  }
}

function buildTree(groups, hosts) {
  const nodes = new Map()
  const roots = []
  groups.forEach((group) => {
    nodes.set(group.id, { id: `group-${group.id}`, rawId: group.id, parentId: group.parentId, type: 'group', label: group.name, children: [] })
  })
  nodes.forEach((node) => {
    if (node.parentId && nodes.has(node.parentId)) nodes.get(node.parentId).children.push(node)
    else roots.push(node)
  })
  hosts.forEach((host) => {
    const hostNode = { id: `host-${host.id}`, rawId: host.id, type: 'host', label: host.hostName || host.sshIp, host, children: [] }
    const hostGroups = Array.isArray(host.hostGroups) && host.hostGroups.length ? host.hostGroups : host.groupId ? [{ id: host.groupId }] : []
    if (hostGroups.length) {
      hostGroups.forEach((item) => {
        const group = nodes.get(item.id)
        if (group) group.children.push({ ...hostNode, id: `${hostNode.id}-g-${item.id}` })
      })
    } else roots.push(hostNode)
  })
  return roots
}

function handleNodeClick(node) {
  if (node.type === 'host') openHost(node.host)
}

function getSessionId(host) { return `host-${host.id}` }

async function openHost(host) {
  const id = getSessionId(host)
  if (sessions.value.some((item) => item.id === id)) return activateSession(id)
  sessions.value.push({ id, host, status: 'connecting' })
  activeSessionId.value = id
  await nextTick()
  bindTerminalResize()
  createTerminal(id)
  connectSocket(id)
}

function setTerminalElement(id, element) {
  if (element) terminalElements.set(id, element)
  else terminalElements.delete(id)
}

function createTerminal(id) {
  const element = terminalElements.get(id)
  if (!element || runtimes.has(id)) return
  const term = new Terminal({
    cursorBlink: true,
    convertEol: true,
    rows: 34,
    cols: 150,
    fontSize: 13,
    fontFamily: 'Consolas, "Courier New", monospace',
    theme: { background: '#050000', foreground: '#e6edf3', cursor: '#00ff88', green: '#00ff88', brightGreen: '#23ff9a', red: '#ff4d4f' }
  })
  const runtime = { term, socket: undefined, inputDisposable: undefined }
  runtimes.set(id, runtime)
  term.open(element)
  runtime.inputDisposable = term.onData((data) => {
    if (runtime.socket?.readyState === WebSocket.OPEN) runtime.socket.send(data)
  })
  term.focus()
  scheduleTerminalSizeSync()
}

function updateSession(id, patch) {
  const session = sessions.value.find((item) => item.id === id)
  if (session) Object.assign(session, patch)
}

function connectSocket(id) {
  const session = sessions.value.find((item) => item.id === id)
  const runtime = runtimes.get(id)
  if (!session || !runtime) return
  disconnectSession(id, false)
  updateSession(id, { status: 'connecting' })
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const token = encodeURIComponent(getToken())
  const url = `${protocol}://${window.location.host}/api/v1/asset/terminal/ws?hostId=${session.host.id}&rows=${runtime.term.rows || 34}&cols=${runtime.term.cols || 150}&token=${token}`
  const currentSocket = new WebSocket(url)
  runtime.socket = currentSocket
  currentSocket.onopen = () => {
    if (runtime.socket !== currentSocket) return
    updateSession(id, { status: 'connected' })
    runtime.term.writeln(`\x1b[32m${at('sshWelcome', { host: session.host.sshIp || session.host.hostName })}\x1b[0m`)
    if (activeSessionId.value === id) {
      scheduleTerminalSizeSync()
      runtime.term.focus()
    }
  }
  currentSocket.onmessage = (event) => {
    if (runtime.socket === currentSocket) runtime.term.write(event.data)
  }
  currentSocket.onerror = () => {
    if (runtime.socket !== currentSocket) return
    updateSession(id, { status: 'error' })
    runtime.term.writeln(`\r\n\x1b[31m${at('sshConnectError')}\x1b[0m`)
  }
  currentSocket.onclose = () => {
    if (runtime.socket !== currentSocket) return
    runtime.socket = undefined
    updateSession(id, { status: 'disconnected' })
    runtime.term.writeln(`\r\n\x1b[33m${at('disconnected')}\x1b[0m`)
  }
}

function disconnectSession(id, showMessage = true) {
  const runtime = runtimes.get(id)
  if (!runtime?.socket) return
  const currentSocket = runtime.socket
  runtime.socket = undefined
  currentSocket.onopen = null
  currentSocket.onmessage = null
  currentSocket.onerror = null
  currentSocket.onclose = null
  currentSocket.close()
  updateSession(id, { status: 'disconnected' })
  if (showMessage) runtime.term.writeln(`\r\n\x1b[33m${at('disconnected')}\x1b[0m`)
}

function activateSession(id) {
  activeSessionId.value = id
  nextTick(() => {
    scheduleTerminalSizeSync()
    runtimes.get(id)?.term.focus()
  })
}

function reconnect() {
  if (!activeSession.value) return ElMessage.warning(at('selectHostFirst'))
  connectSocket(activeSession.value.id)
}
function disconnect() { if (activeSession.value) disconnectSession(activeSession.value.id) }
function clearTerminal() { if (activeSession.value) runtimes.get(activeSession.value.id)?.term.clear() }

function closeSession(id = activeSessionId.value) {
  const index = sessions.value.findIndex((item) => item.id === id)
  if (index < 0) return
  disposeRuntime(id)
  sessions.value.splice(index, 1)
  if (activeSessionId.value === id) {
    activeSessionId.value = sessions.value[index]?.id || sessions.value[index - 1]?.id
    nextTick(() => {
      scheduleTerminalSizeSync()
      activeSessionId.value && runtimes.get(activeSessionId.value)?.term.focus()
    })
  }
}

function closeOtherSessions() {
  if (!activeSessionId.value) return
  sessions.value.filter((item) => item.id !== activeSessionId.value).forEach((item) => disposeRuntime(item.id))
  sessions.value = sessions.value.filter((item) => item.id === activeSessionId.value)
  ElMessage.success(at('otherSessionsClosed'))
}

function disposeRuntime(id) {
  const runtime = runtimes.get(id)
  if (!runtime) return
  disconnectSession(id, false)
  runtime.inputDisposable?.dispose()
  runtime.term?.dispose()
  runtimes.delete(id)
  terminalElements.delete(id)
}

function closeAllSessions() {
  sessions.value.forEach((item) => disposeRuntime(item.id))
  sessions.value = []
  activeSessionId.value = undefined
}

function bindTerminalResize() {
  if (!terminalBoxRef.value || resizeObserver) return
  resizeObserver = new ResizeObserver(scheduleTerminalSizeSync)
  resizeObserver.observe(terminalBoxRef.value)
}
function scheduleTerminalSizeSync() {
  cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(syncTerminalSize)
}
function syncTerminalSize() {
  const id = activeSessionId.value
  const runtime = id && runtimes.get(id)
  const element = id && terminalElements.get(id)
  if (!runtime || !element) return
  const style = window.getComputedStyle(element)
  const width = element.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)
  const height = element.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom)
  if (!width || !height) return
  const screen = element.querySelector('.xterm-screen')
  const cellWidth = screen?.clientWidth ? screen.clientWidth / runtime.term.cols : 8.2
  const cellHeight = screen?.clientHeight ? screen.clientHeight / runtime.term.rows : 18
  const cols = Math.max(80, Math.floor(width / cellWidth))
  const rows = Math.max(20, Math.floor(height / cellHeight))
  if (runtime.term.cols !== cols || runtime.term.rows !== rows) runtime.term.resize(cols, rows)
}

function handleTabClosed(event) { if (event.detail?.path === '/assets/terminal') closeAllSessions() }
onMounted(() => {
  window.addEventListener('ops-admin:tab-closed', handleTabClosed)
  loadTree()
})
onActivated(() => {
  bindTerminalResize()
  scheduleTerminalSizeSync()
  activeSessionId.value && runtimes.get(activeSessionId.value)?.term.focus()
})
onBeforeUnmount(() => {
  window.removeEventListener('ops-admin:tab-closed', handleTabClosed)
  resizeObserver?.disconnect()
  cancelAnimationFrame(resizeFrame)
  closeAllSessions()
})
</script>

<template>
  <div class="terminal-page">
    <aside class="asset-tree-card">
      <h3>{{ at('assetGroupTitle') }}</h3>
      <el-input v-model="query.keyword" clearable :placeholder="at('groupHostSearchPlaceholder')" class="tree-search" @keyup.enter="loadTree" @clear="loadTree" />
      <el-tree ref="treeRef" v-loading="loading" :data="groupTree" node-key="id" default-expand-all :expand-on-click-node="false" class="asset-tree" @node-click="handleNodeClick">
        <template #default="{ data }">
          <span :class="['tree-node', data.type, { active: data.type === 'host' && activeSession?.host.id === data.host?.id }]">
            <el-icon v-if="data.type === 'group'"><Folder /></el-icon>
            <el-icon v-else><Monitor /></el-icon>
            <span>{{ data.label }}</span>
          </span>
        </template>
      </el-tree>
    </aside>

    <section v-if="sessions.length" ref="terminalBoxRef" class="terminal-window">
      <header class="terminal-titlebar">
        <div><strong>{{ at('sshTerminalTitle') }}</strong><span class="session-count">{{ at('openSessionCount', { count: sessions.length }) }}</span></div>
        <el-dropdown trigger="click" @command="(command) => command === 'closeOthers' && closeOtherSessions()">
          <button class="terminal-menu" :aria-label="at('terminalSessionActions')">•••</button>
          <template #dropdown><el-dropdown-menu><el-dropdown-item command="closeOthers" :disabled="sessions.length < 2">{{ at('closeOtherSessions') }}</el-dropdown-item></el-dropdown-menu></template>
        </el-dropdown>
      </header>
      <div class="terminal-tabs" role="tablist" :aria-label="at('terminalSessions')">
        <button v-for="session in sessions" :key="session.id" :class="['terminal-tab', { active: session.id === activeSessionId }]" role="tab" :aria-selected="session.id === activeSessionId" @click="activateSession(session.id)">
          <span :class="['connection-dot', session.status]" />
          <span class="terminal-tab-label">{{ session.host.sshIp || session.host.hostName }}</span>
          <span class="terminal-tab-close" :title="at('closeTerminal')" @click.stop="closeSession(session.id)">×</span>
        </button>
      </div>
      <div class="terminal-toolbar">
        <span class="active-host">{{ activeSession?.host.hostName || activeSession?.host.sshIp }}</span>
        <div class="terminal-actions">
          <el-button size="small" color="#00d084" plain @click="reconnect">{{ at('reconnect') }}</el-button>
          <el-button size="small" color="#00d084" plain @click="disconnect">{{ at('disconnect') }}</el-button>
          <el-button size="small" color="#00d084" plain @click="clearTerminal">{{ at('clearScreenButton') }}</el-button>
          <el-button size="small" type="danger" plain @click="closeSession()">{{ at('closeButton') }}</el-button>
        </div>
      </div>
      <div v-for="session in sessions" :key="`screen-${session.id}`" v-show="session.id === activeSessionId" class="terminal-stage">
        <div :ref="(element) => setTerminalElement(session.id, element)" class="terminal-body" />
      </div>
    </section>

    <section v-else class="terminal-empty"><div><h2>{{ at('terminalLoginTitle') }}</h2><p>{{ at('terminalLoginDesc') }}</p></div></section>
  </div>
</template>

<style scoped>
.terminal-page { display: grid; grid-template-columns: 300px minmax(0, 1fr); gap: 12px; height: calc(100vh - 190px); min-height: 680px; }
.asset-tree-card { padding: 28px 20px; border-radius: 4px; background: #142230; color: #00b96b; box-shadow: 0 18px 40px rgba(18, 33, 49, .18); }
.asset-tree-card h3 { margin: 0 0 18px; color: #00c875; font-size: 20px; }.tree-search { margin-bottom: 18px; }
.asset-tree { --el-tree-node-hover-bg-color: rgba(0, 185, 107, .1); background: transparent; color: #00b96b; }
.tree-node { display: inline-flex; align-items: center; gap: 8px; border-radius: 4px; font-weight: 700; }.tree-node.host { color: #a3b600; }.tree-node.host.active { color: #16d995; }
.terminal-window { display: flex; flex-direction: column; min-height: 0; overflow: hidden; border-radius: 12px 12px 0 0; background: #20384c; box-shadow: 0 22px 48px rgba(7, 20, 35, .28); }
.terminal-titlebar { display: flex; align-items: center; justify-content: space-between; min-height: 58px; padding: 0 18px 0 22px; border-bottom: 1px solid rgba(0, 208, 132, .6); color: #00ff88; font-size: 18px; }.session-count { margin-left: 12px; color: #9db0c1; font-size: 12px; font-weight: 500; }
.terminal-menu { width: 32px; height: 30px; border: 1px solid rgba(157, 176, 193, .45); border-radius: 6px; background: transparent; color: #c7d4dc; cursor: pointer; font-weight: 700; letter-spacing: 1px; }.terminal-menu:hover { border-color: #00d084; color: #00ff88; }
.terminal-tabs { display: flex; min-height: 44px; gap: 4px; overflow-x: auto; padding: 6px 10px 0; border-bottom: 1px solid rgba(111, 140, 164, .35); background: #172b3a; }
.terminal-tab { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 8px; max-width: 240px; padding: 0 10px; border: 1px solid transparent; border-bottom: 0; border-radius: 7px 7px 0 0; background: transparent; color: #aabac7; cursor: pointer; font-size: 13px; }.terminal-tab:hover { background: rgba(0, 208, 132, .08); color: #ecf6f1; }.terminal-tab.active { border-color: rgba(0, 208, 132, .55); background: #20384c; color: #f4fffa; }.terminal-tab-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.connection-dot { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: #8393a5; }.connection-dot.connecting { background: #f2c94c; box-shadow: 0 0 0 3px rgba(242, 201, 76, .14); }.connection-dot.connected { background: #00d084; box-shadow: 0 0 0 3px rgba(0, 208, 132, .14); }.connection-dot.error { background: #ff6b6b; }
.terminal-tab-close { display: inline-grid; width: 18px; height: 18px; place-items: center; border-radius: 4px; color: #91a2b1; font-size: 18px; line-height: 1; }.terminal-tab-close:hover { background: rgba(255, 77, 79, .18); color: #ff8a8c; }
.terminal-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 52px; padding: 0 12px; border-bottom: 1px solid rgba(0, 208, 132, .6); }.active-host { overflow: hidden; color: #d6e4ec; font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }.terminal-actions { display: flex; flex: 0 0 auto; gap: 10px; }
.terminal-stage { display: flex; flex: 1; min-height: 0; }.terminal-body { display: flex; flex: 1; min-height: 0; padding: 10px 12px; box-sizing: border-box; background: #050000; overflow: hidden; }.terminal-body :deep(.xterm) { width: 100%; height: 100%; }
.terminal-empty { display: grid; place-items: center; border-radius: 12px; background: linear-gradient(135deg, #243d70, #466df4); color: #fff; }.terminal-empty h2 { margin: 0 0 10px; font-size: 34px; }.terminal-empty p { max-width: 580px; margin: 0; color: rgba(255, 255, 255, .8); line-height: 1.7; }
@media (max-width: 960px) { .terminal-page { grid-template-columns: 240px minmax(0, 1fr); }.terminal-toolbar { align-items: flex-start; flex-direction: column; padding: 10px 12px; } }
</style>
