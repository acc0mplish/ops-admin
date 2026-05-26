<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Back, Monitor } from '@element-plus/icons-vue'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { buildK8sPodTerminalWSUrl, queryK8sPodContainers } from '../../api/k8s'

const route = useRoute()
const router = useRouter()

const clusterId = computed(() => Number(route.params.clusterId || 0))
const namespace = computed(() => String(route.params.namespace || ''))
const podName = computed(() => String(route.params.podName || ''))

const terminalRef = ref()
const terminalBoxRef = ref()
const containers = ref([])
const selectedContainer = ref(String(route.query.container || ''))
const connecting = ref(false)
const connected = ref(false)

let term
let socket
let inputDisposable
let resizeObserver

function createTerminal() {
  term = new Terminal({
    cursorBlink: true,
    convertEol: true,
    fontSize: 13,
    rows: 32,
    cols: 120,
    fontFamily: 'Consolas, "Courier New", monospace',
    theme: {
      background: '#07111f',
      foreground: '#e5edf7',
      cursor: '#67c23a',
      green: '#67c23a',
      brightGreen: '#95d475',
      red: '#f56c6c'
    }
  })
  term.open(terminalRef.value)
  term.focus()
  inputDisposable = term.onData((data) => {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ operation: 'stdin', data }))
    }
  })
  bindTerminalResize()
}

function bindTerminalResize() {
  if (!terminalBoxRef.value || !term) return
  resizeObserver = new ResizeObserver(() => {
    syncTerminalSize()
  })
  resizeObserver.observe(terminalBoxRef.value)
  syncTerminalSize()
}

function syncTerminalSize() {
  if (!term || !terminalBoxRef.value) return
  const width = terminalBoxRef.value.clientWidth
  const height = terminalBoxRef.value.clientHeight
  if (!width || !height) return
  const cols = Math.max(80, Math.floor((width - 24) / 8.2))
  const rows = Math.max(20, Math.floor((height - 24) / 18))
  if (term.cols !== cols || term.rows !== rows) {
    term.resize(cols, rows)
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(
        JSON.stringify({
          operation: 'resize',
          data: { cols, rows }
        })
      )
    }
  }
}

async function loadContainers() {
  const list = await queryK8sPodContainers(clusterId.value, namespace.value, podName.value)
  containers.value = Array.isArray(list) ? list : []
  if (!containers.value.length) {
    ElMessage.warning('当前 Pod 没有可用容器')
    return
  }
  if (!containers.value.includes(selectedContainer.value)) {
    selectedContainer.value = containers.value[0]
  }
}

function connectTerminal() {
  if (!selectedContainer.value) {
    ElMessage.warning('请先选择容器')
    return
  }
  disconnectTerminal()
  connecting.value = true
  const url = buildK8sPodTerminalWSUrl({
    clusterId: clusterId.value,
    namespace: namespace.value,
    podName: podName.value,
    container: selectedContainer.value,
    rows: term?.rows || 32,
    cols: term?.cols || 120
  })
  socket = new WebSocket(url)
  socket.onopen = () => {
    connecting.value = false
    connected.value = true
    term?.clear()
    term?.writeln(`\x1b[32m已连接到 ${namespace.value}/${podName.value}\x1b[0m`)
    term?.writeln(`\x1b[36m容器: ${selectedContainer.value}\x1b[0m`)
    term?.writeln('')
    syncTerminalSize()
  }
  socket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload?.operation === 'stdout' && payload.data) {
        term?.write(payload.data)
        return
      }
    } catch (error) {
      // ignore non-json payload
    }
    term?.write(event.data)
  }
  socket.onerror = () => {
    connecting.value = false
    connected.value = false
    ElMessage.error('Pod 终端连接失败')
  }
  socket.onclose = () => {
    connecting.value = false
    connected.value = false
    term?.writeln('\r\n\x1b[33m连接已关闭。\x1b[0m')
  }
}

function disconnectTerminal() {
  if (socket) {
    socket.onclose = null
    socket.close()
    socket = undefined
  }
  connected.value = false
  connecting.value = false
}

function clearTerminal() {
  term?.clear()
}

function handleContainerChange() {
  connectTerminal()
}

function goBack() {
  router.push('/assets/k8s/pods')
}

function disposeTerminal() {
  resizeObserver?.disconnect()
  resizeObserver = undefined
  inputDisposable?.dispose()
  inputDisposable = undefined
  disconnectTerminal()
  term?.dispose()
  term = undefined
}

onMounted(async () => {
  await nextTick()
  createTerminal()
  try {
    await loadContainers()
    if (selectedContainer.value) {
      connectTerminal()
    }
  } catch (error) {
    ElMessage.error(error.message || '获取 Pod 容器失败')
  }
})

onBeforeUnmount(() => {
  disposeTerminal()
})
</script>

<template>
  <div class="pod-terminal-page">
    <section class="terminal-shell">
      <header class="terminal-head">
        <div class="title-row">
          <el-button text @click="goBack">
            <el-icon><Back /></el-icon>
            返回 Pod 管理
          </el-button>
          <div class="title-block">
            <span class="title-label">
              <el-icon><Monitor /></el-icon>
              Pod 终端
            </span>
            <strong>{{ namespace }}/{{ podName }}</strong>
          </div>
        </div>

        <div class="toolbar">
          <el-select
            v-model="selectedContainer"
            class="container-select"
            placeholder="选择容器"
            filterable
            :disabled="!containers.length"
            @change="handleContainerChange"
          >
            <el-option v-for="item in containers" :key="item" :label="item" :value="item" />
          </el-select>
          <el-button :loading="connecting" type="primary" @click="connectTerminal">连接</el-button>
          <el-button @click="disconnectTerminal">断开</el-button>
          <el-button @click="clearTerminal">清屏</el-button>
        </div>
      </header>

      <div ref="terminalBoxRef" class="terminal-stage">
        <div ref="terminalRef" class="terminal-body" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.pod-terminal-page {
  min-height: calc(100vh - 190px);
}

.terminal-shell {
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 190px);
  border: 1px solid #dbe5f0;
  border-radius: 8px;
  background: #fff;
  overflow: hidden;
}

.terminal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid #e6edf5;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.title-block {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.title-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #6b7280;
  font-size: 13px;
}

.title-block strong {
  color: #111827;
  font-size: 18px;
  line-height: 1.3;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.container-select {
  width: 220px;
}

.terminal-stage {
  flex: 1;
  min-height: 560px;
  padding: 12px;
  background: #07111f;
}

.terminal-body {
  width: 100%;
  height: 100%;
}

@media (max-width: 960px) {
  .terminal-head {
    flex-direction: column;
    align-items: stretch;
  }

  .title-row,
  .toolbar {
    width: 100%;
  }

  .toolbar {
    flex-wrap: wrap;
  }

  .container-select {
    width: 100%;
  }
}
</style>
