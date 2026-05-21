<script setup>
import { nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { ElMessage } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import { getToken } from '../../utils/auth'

const loading = ref(false)
const treeRef = ref()
const terminalRef = ref()
const terminalBoxRef = ref()
const groupTree = ref([])
const activeHost = ref()
const query = reactive({ keyword: '' })

let term
let socket
let inputDisposable

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
    nodes.set(group.id, {
      id: `group-${group.id}`,
      rawId: group.id,
      parentId: group.parentId,
      type: 'group',
      label: group.name,
      children: []
    })
  })
  nodes.forEach((node) => {
    if (node.parentId && nodes.has(node.parentId)) {
      nodes.get(node.parentId).children.push(node)
    } else {
      roots.push(node)
    }
  })
  hosts.forEach((host) => {
    const hostNode = {
      id: `host-${host.id}`,
      rawId: host.id,
      type: 'host',
      label: host.hostName || host.sshIp,
      host,
      children: []
    }
    const group = nodes.get(host.groupId)
    if (group) {
      group.children.push(hostNode)
    } else {
      roots.push(hostNode)
    }
  })
  return roots
}

function handleNodeClick(node) {
  if (node.type !== 'host') return
  connectHost(node.host)
}

async function connectHost(host) {
  activeHost.value = host
  closeTerminal()
  await nextTick()
  createTerminal()
  connectSocket(host)
}

function createTerminal() {
  term = new Terminal({
    cursorBlink: true,
    convertEol: true,
    rows: 34,
    cols: 150,
    fontSize: 13,
    fontFamily: 'Consolas, "Courier New", monospace',
    theme: {
      background: '#050000',
      foreground: '#e6edf3',
      cursor: '#00ff88',
      green: '#00ff88',
      brightGreen: '#23ff9a',
      red: '#ff4d4f'
    }
  })
  term.open(terminalRef.value)
  term.focus()
  inputDisposable = term.onData((data) => {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(data)
    }
  })
}

function connectSocket(host) {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const token = encodeURIComponent(getToken())
  const url = `${protocol}://${window.location.host}/api/v1/asset/terminal/ws?hostId=${host.id}&rows=34&cols=150&token=${token}`
  socket = new WebSocket(url)
  socket.onopen = () => {
    term?.writeln(`\x1b[32m欢迎使用SSH终端，正在连接 ${host.sshIp || host.hostName} ...\x1b[0m`)
  }
  socket.onmessage = (event) => {
    term?.write(event.data)
  }
  socket.onerror = () => {
    term?.writeln('\r\n\x1b[31mSSH连接异常，请检查主机、端口和认证凭据。\x1b[0m')
  }
  socket.onclose = () => {
    term?.writeln('\r\n\x1b[33m连接已断开。\x1b[0m')
  }
}

function reconnect() {
  if (!activeHost.value) {
    ElMessage.warning('请先选择一台主机')
    return
  }
  connectHost(activeHost.value)
}

function disconnect() {
  if (socket) {
    socket.close()
  }
}

function clearTerminal() {
  term?.clear()
}

function closePanel() {
  disconnect()
  closeTerminal()
  activeHost.value = undefined
}

function closeTerminal() {
  inputDisposable?.dispose()
  inputDisposable = undefined
  if (socket) {
    socket.onclose = null
    socket.close()
    socket = undefined
  }
  term?.dispose()
  term = undefined
}

onMounted(loadTree)
onBeforeUnmount(closeTerminal)
</script>

<template>
  <div class="terminal-page">
    <aside class="asset-tree-card">
      <h3>资产分组</h3>
      <el-input
        v-model="query.keyword"
        clearable
        placeholder="搜索分组 / 主机"
        class="tree-search"
        @keyup.enter="loadTree"
        @clear="loadTree"
      />
      <el-tree
        ref="treeRef"
        v-loading="loading"
        :data="groupTree"
        node-key="id"
        default-expand-all
        :expand-on-click-node="false"
        class="asset-tree"
        @node-click="handleNodeClick"
      >
        <template #default="{ data }">
          <span :class="['tree-node', data.type]">
            <el-icon v-if="data.type === 'group'"><Folder /></el-icon>
            <el-icon v-else><Monitor /></el-icon>
            <span>{{ data.label }}</span>
          </span>
        </template>
      </el-tree>
    </aside>

    <section v-if="activeHost" ref="terminalBoxRef" class="terminal-window">
      <header class="terminal-titlebar">
        <strong>SSH终端 - {{ activeHost.sshIp || activeHost.hostName }}</strong>
        <button class="close-x" @click="closePanel">×</button>
      </header>
      <div class="terminal-toolbar">
        <el-button size="small" color="#00d084" plain @click="reconnect">重新连接</el-button>
        <el-button size="small" color="#00d084" plain @click="disconnect">断开</el-button>
        <el-button size="small" color="#00d084" plain @click="clearTerminal">清屏</el-button>
        <el-button size="small" type="danger" plain @click="closePanel">关闭</el-button>
      </div>
      <div ref="terminalRef" class="terminal-body" />
    </section>

    <section v-else class="terminal-empty">
      <div>
        <h2>终端登录</h2>
        <p>请在左侧资产分组中选择一台主机，系统会使用主机关联的认证凭据发起 SSH 连接。</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.terminal-page {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 12px;
  min-height: calc(100vh - 190px);
}

.asset-tree-card {
  padding: 28px 20px;
  border-radius: 4px;
  background: #142230;
  color: #00b96b;
  box-shadow: 0 18px 40px rgba(18, 33, 49, 0.18);
}

.asset-tree-card h3 {
  margin: 0 0 18px;
  color: #00c875;
  font-size: 20px;
}

.tree-search {
  margin-bottom: 18px;
}

.asset-tree {
  --el-tree-node-hover-bg-color: rgba(0, 185, 107, 0.1);
  background: transparent;
  color: #00b96b;
}

.tree-node {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
}

.tree-node.host {
  color: #a3b600;
}

.terminal-window {
  overflow: hidden;
  border-radius: 12px 12px 0 0;
  background: #20384c;
  box-shadow: 0 22px 48px rgba(7, 20, 35, 0.28);
}

.terminal-titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  padding: 0 22px;
  border-bottom: 1px solid #00d084;
  color: #00ff88;
  font-size: 18px;
}

.close-x {
  border: 0;
  background: transparent;
  color: #ff4d4f;
  cursor: pointer;
  font-size: 26px;
  font-weight: 700;
}

.terminal-toolbar {
  display: flex;
  gap: 12px;
  padding: 12px;
  border-bottom: 1px solid #00d084;
}

.terminal-body {
  height: calc(100vh - 330px);
  min-height: 520px;
  padding: 10px 12px;
  background: #050000;
}

.terminal-empty {
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: linear-gradient(135deg, #243d70, #466df4);
  color: #fff;
}

.terminal-empty h2 {
  margin: 0 0 10px;
  font-size: 34px;
}

.terminal-empty p {
  margin: 0;
  color: rgba(255, 255, 255, 0.8);
}
</style>
