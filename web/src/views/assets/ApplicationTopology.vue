<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import { queryAssetServiceRuntimeTopology } from '../../api/asset'
import ServiceWorkloadDetail from './ServiceWorkloadDetail.vue'
import ServiceWorkloadLogs from './ServiceWorkloadLogs.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const topology = ref({})
const detailVisible = ref(false)
const activeDrawerTab = ref('detail')
const selectedWorkload = ref(null)
const selectedLog = ref(null)
const serviceId = computed(() => Number(route.query.serviceId))
const workloads = computed(() => topology.value.workloads || [])
const normalize = (value = '') => String(value).toLowerCase()
const find = (predicate) => workloads.value.find((item) => predicate(normalize(item.name)))
function healthy(item) {
  if (!item) return true
  const [ready, expected] = String(item.ready || '').split('/').map(Number)
  return expected > 0 && ready === expected && Number(item.available) >= expected
}
const nodes = computed(() => {
  const add = (id, title, x, y, kind, note, workload = null) => ({ id, title, x, y, kind, note, workload, healthy: healthy(workload) })
  const nginx = find((name) => name.includes('nginx-gm')); const mgr = find((name) => name.includes('mgr')); const gate = find((name) => name.includes('gate')); const login = find((name) => name.includes('login')); const notice = find((name) => name.includes('notice')); const home = find((name) => name.includes('home')); const world = find((name) => name.includes('world')); const zk = find((name) => name.includes('zookeeper') || name === 'zk')
  const result = [add('player', '玩家客户端', 34, 270, 'external', '仅通过 Gate / Notice 通信')]
  if (nginx) result.push(add('nginx', nginx.name, 250, 74, 'management', '独立 GM 入口', nginx))
  if (mgr) result.push(add('mgr', mgr.name, 460, 74, 'management', 'GM 后台', mgr))
  if (gate) result.push(add('gate', gate.name, 270, 280, 'public', 'WebSocket · 对外入口', gate))
  if (login) result.push(add('login', login.name, 505, 280, 'service', '登录校验', login))
  result.push(add('zk', zk?.name || 'ZooKeeper', 740, 280, 'dependency', '服务注册 / 启用状态', zk || null))
  if (home) result.push(add('home', home.name, 975, 190, 'service', '游戏服务', home))
  if (world) result.push(add('world', world.name, 975, 370, 'service', '游戏服务', world))
  if (notice) result.push(add('notice', notice.name, 270, 470, 'public', '日志上报 · 对外入口', notice))
  const known = new Set([nginx, mgr, gate, login, notice, home, world, zk].filter(Boolean).map((item) => item.name))
  workloads.value.filter((item) => !known.has(item.name)).forEach((item, index) => result.push(add(`extra-${index}`, item.name, 740 + (index % 2) * 230, 74 + Math.floor(index / 2) * 105, 'service', `${item.type} · Ready ${item.ready || '0/0'}`, item)))
  return result
})
const positions = computed(() => Object.fromEntries(nodes.value.map((item) => [item.id, item])))
const edges = computed(() => {
  const result = []; const has = (id) => Boolean(positions.value[id]); const add = (from, to, label, tone = 'default') => { if (has(from) && has(to)) result.push({ from, to, label, tone }) }
  add('player', 'gate', 'WebSocket 登录', 'public'); add('player', 'notice', '日志上报', 'public'); add('gate', 'login', '登录请求', 'public'); add('login', 'zk', '读取启用服务'); add('zk', 'home', '服务注册'); add('zk', 'world', '服务注册'); add('gate', 'home', '游戏会话'); add('gate', 'world', '游戏会话'); add('mgr', 'zk', '启停管理', 'management'); add('mgr', 'home', '管理游戏服', 'management'); add('mgr', 'world', '管理游戏服', 'management')
  nodes.value.filter((item) => !['player', 'nginx', 'zk'].includes(item.id)).forEach((item) => add(item.id, 'nginx', '访问 GM', 'management'))
  return result
})
function path(edge) { const from = positions.value[edge.from]; const to = positions.value[edge.to]; const x1 = from.x + 174; const y1 = from.y + 46; const x2 = to.x; const y2 = to.y + 46; const middle = Math.round((x1 + x2) / 2); return `M ${x1} ${y1} H ${middle} V ${y2} H ${x2}` }
function label(edge) { const from = positions.value[edge.from]; const to = positions.value[edge.to]; return { x: Math.round((from.x + 174 + to.x) / 2), y: Math.round((from.y + to.y) / 2) - 8 } }
function openDetail(node) { if (!node.workload) return; selectedWorkload.value = node.workload; selectedLog.value = null; activeDrawerTab.value = 'detail'; detailVisible.value = true }
function openLogs(target = null) { selectedLog.value = target; activeDrawerTab.value = 'logs' }
async function load() { if (!serviceId.value) return; loading.value = true; try { topology.value = await queryAssetServiceRuntimeTopology(serviceId.value) } finally { loading.value = false } }
onMounted(load)
</script>

<template>
  <div class="service-topology" v-loading="loading">
    <section class="topology-header"><div><el-button text :icon="ArrowLeft" @click="router.push('/assets/services')">返回服务管理</el-button><p class="eyebrow">SERVICE RESOURCE TOPOLOGY</p><h1>{{ topology.service?.name || '服务资源拓扑' }}</h1><p><code>{{ topology.service?.serviceUid }}</code> · {{ topology.cluster?.name || '未绑定集群' }} / {{ topology.namespace || '-' }}</p></div><el-button :icon="Refresh" @click="load">刷新运行状态</el-button></section>
    <el-alert v-if="topology.refreshError" type="warning" :closable="false" show-icon :title="topology.refreshError" />
    <section class="topology-card"><header><div><h2>业务通信链路</h2><p>绿色 ✓ 表示健康，红色 ! 表示异常。点击服务节点进入服务详情。</p></div><div class="legend"><span class="public"></span>对外入口 <span class="management"></span>GM 管理 <span class="service"></span>内部服务 <i class="legend-health ok">✓</i>健康 <i class="legend-health bad">!</i>异常</div></header><div class="canvas"><svg viewBox="0 0 1200 600"><defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#8ca1bf" /></marker></defs><g v-for="edge in edges" :key="`${edge.from}-${edge.to}`"><path :d="path(edge)" :class="['edge', edge.tone]" marker-end="url(#arrow)"/><text :x="label(edge).x" :y="label(edge).y" text-anchor="middle">{{ edge.label }}</text></g></svg><button v-for="node in nodes" :key="node.id" class="topology-node" :class="[`node-${node.kind}`, { clickable: node.workload }]" :style="{ left: `${node.x}px`, top: `${node.y}px` }" @click="openDetail(node)"><i v-if="node.workload" class="health-badge" :class="node.healthy ? 'is-ok' : 'is-bad'">{{ node.healthy ? '✓' : '!' }}</i><b>{{ node.title }}</b><small>{{ node.note }}</small><small v-if="node.workload">{{ node.workload.type }} · Ready {{ node.workload.ready || '0/0' }}</small></button></div></section>
    <section class="workload-card"><h2>关联工作负载</h2><el-table :data="workloads" size="small"><el-table-column prop="name" label="工作负载"/><el-table-column prop="type" label="类型" width="120"/><el-table-column prop="ready" label="Ready" width="100"/><el-table-column label="健康" width="100"><template #default="{ row }"><el-tag :type="healthy(row) ? 'success' : 'danger'">{{ healthy(row) ? '✓ 正常' : '! 异常' }}</el-tag></template></el-table-column><el-table-column label="操作" width="110"><template #default="{ row }"><el-button link type="primary" @click="openDetail({ workload: row })">服务详情</el-button></template></el-table-column></el-table></section>
    <el-drawer v-model="detailVisible" size="70%" :with-header="false" destroy-on-close>
      <div v-if="selectedWorkload" class="service-drawer-tabs"><el-button :type="activeDrawerTab === 'detail' ? 'primary' : 'default'" @click="activeDrawerTab = 'detail'">服务详情</el-button><el-button :type="activeDrawerTab === 'logs' ? 'primary' : 'default'" @click="openLogs()">服务日志</el-button></div>
      <ServiceWorkloadDetail v-if="activeDrawerTab === 'detail'" :service-id="serviceId" :workload-type="selectedWorkload.type" :workload-name="selectedWorkload.name" inline @close="detailVisible = false" @show-logs="openLogs" />
      <ServiceWorkloadLogs v-else :key="selectedLog?.podName || 'default'" :service-id="serviceId" :workload-type="selectedWorkload.type" :workload-name="selectedWorkload.name" :pod-name="selectedLog?.podName || ''" inline @close="detailVisible = false" />
    </el-drawer>
  </div>
</template>

<style scoped>
.service-topology{padding:24px;background:#f4f7fc;min-height:100%}.topology-header,.topology-card,.workload-card{background:#fff;border:1px solid #e0e8f5;border-radius:18px}.topology-header{display:flex;justify-content:space-between;align-items:center;padding:22px 28px;margin-bottom:16px;background:linear-gradient(120deg,#fff,#eef4ff)}.eyebrow{font-size:12px;letter-spacing:.08em;color:#3970df;font-weight:800;margin:10px 0 4px}.topology-header h1{margin:0;color:#102b54}.topology-header p{margin:8px 0 0;color:#7485a0}.topology-header code{color:#4168b5}.topology-card{overflow:hidden;margin-top:16px}.topology-card>header{display:flex;justify-content:space-between;align-items:center;padding:20px 24px;border-bottom:1px solid #e7edf7}.topology-card h2,.workload-card h2{margin:0;color:#142e57;font-size:18px}.topology-card p{margin:7px 0 0;color:#7687a1;font-size:13px}.legend{display:flex;gap:8px;align-items:center;flex-wrap:wrap;color:#7586a0;font-size:12px}.legend span{width:10px;height:10px;border-radius:50%;margin-left:8px}.legend .public{background:#2585ee}.legend .management{background:#8856e9}.legend .service{background:#18ad82}.legend-health{width:18px;height:18px;border-radius:50%;display:grid;place-items:center;color:#fff;font-style:normal;font-weight:800}.legend-health.ok{background:#16b68a}.legend-health.bad{background:#ef5259}.canvas{position:relative;min-width:1200px;height:600px;background-image:radial-gradient(#d7e3f4 1px,transparent 1px);background-size:18px 18px}.canvas>svg{position:absolute;inset:0;width:1200px;height:600px}.edge{fill:none;stroke:#8ca1bf;stroke-width:1.8}.edge.public{stroke:#2585ee;stroke-width:2.6}.edge.management{stroke:#8856e9;stroke-dasharray:6 4}.canvas text{font-size:11px;fill:#70829e;paint-order:stroke;stroke:#fff;stroke-width:4px}.topology-node{position:absolute;width:174px;min-height:92px;padding:12px 14px;border:2px solid #d5e0f0;border-radius:14px;background:#fff;box-shadow:0 8px 18px rgba(37,62,113,.1);text-align:left}.topology-node.clickable{cursor:pointer}.topology-node.clickable:hover{transform:translateY(-2px);box-shadow:0 10px 22px rgba(37,62,113,.18)}.topology-node b,.topology-node small{display:block}.topology-node b{padding-right:28px;color:#152e56;font-size:14px}.topology-node small{margin-top:5px;color:#7587a4;font-size:11px}.health-badge{position:absolute;right:12px;top:12px;width:19px;height:19px;border-radius:50%;display:grid;place-items:center;color:#fff;font-style:normal;font-size:13px;font-weight:800;line-height:1}.health-badge.is-ok{background:#16b68a}.health-badge.is-bad{background:#ef5259}.node-public{border-color:#55a1f2;background:#f3f9ff}.node-management{border-color:#a27cf0;background:#faf7ff}.node-dependency{border-color:#f0a640;background:#fffaf0}.node-service{border-color:#50caa0;background:#f3fffa}.node-external{border-color:#778eb0;background:#f7f9fd}.workload-card{margin-top:16px;padding:20px}.workload-card h2{margin-bottom:16px}.service-drawer-tabs{position:sticky;top:0;z-index:3;display:flex;gap:10px;padding:14px 24px 0;background:#fff;border-bottom:1px solid #e6edf7}@media(max-width:1200px){.topology-card{overflow:auto}.topology-card>header{align-items:flex-start;gap:12px;flex-direction:column}}
</style>
