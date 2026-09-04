<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import { queryAssetServiceRuntimeTopology } from '../../api/asset'
import ServiceWorkloadDetail from './ServiceWorkloadDetail.vue'
import ServiceWorkloadLogs from './ServiceWorkloadLogs.vue'
import ServicePodMonitor from './ServicePodMonitor.vue'

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
const byInstance = (items) => [...items].sort((left, right) => String(left.name).localeCompare(String(right.name), undefined, { numeric: true, sensitivity: 'base' }))
const homeWorkloads = computed(() => byInstance(workloads.value.filter((item) => normalize(item.name).includes('home'))))
const worldWorkloads = computed(() => byInstance(workloads.value.filter((item) => normalize(item.name).includes('world'))))
const gameColumns = computed(() => Math.min(3, Math.max(1, Math.ceil(Math.sqrt(Math.max(homeWorkloads.value.length, worldWorkloads.value.length, 1))))))
// Keep stateful game workloads clear of ZooKeeper and its dependency paths.
const gameStartX = 1010
const nodeStepX = 214
const nodeStepY = 122
const statefulGroupPadding = 22
const statefulGroupHeaderHeight = 0
const homeRows = computed(() => Math.max(1, Math.ceil(homeWorkloads.value.length / gameColumns.value)))
const worldRows = computed(() => Math.max(1, Math.ceil(worldWorkloads.value.length / gameColumns.value)))
const statefulGroupWidth = computed(() => Math.max(236, gameColumns.value * nodeStepX + statefulGroupPadding * 2 - 28))
const homeGroupY = 122
const homeGroupHeight = computed(() => statefulGroupPadding * 2 + homeRows.value * nodeStepY + 4)
const worldGroupY = computed(() => homeGroupY + homeGroupHeight.value + 58)
const worldGroupHeight = computed(() => statefulGroupPadding * 2 + worldRows.value * nodeStepY + 4)
const worldStartY = computed(() => worldGroupY.value + statefulGroupPadding)
const gameBottomY = computed(() => worldGroupY.value + worldGroupHeight.value)
const canvasWidth = computed(() => Math.max(1240, gameStartX + statefulGroupWidth.value + 30))
const canvasHeight = computed(() => Math.max(600, gameBottomY.value + 130))
const statefulGroups = computed(() => [
  { key: 'home', items: homeWorkloads.value, x: gameStartX - statefulGroupPadding, y: homeGroupY, width: statefulGroupWidth.value, height: homeGroupHeight.value },
  { key: 'world', items: worldWorkloads.value, x: gameStartX - statefulGroupPadding, y: worldGroupY.value, width: statefulGroupWidth.value, height: worldGroupHeight.value }
].filter((group) => group.items.length))
function healthy(item) {
  if (!item) return true
  const [ready, expected] = String(item.ready || '').split('/').map(Number)
  return expected > 0 && ready === expected && Number(item.available) >= expected
}
const nodes = computed(() => {
  const add = (id, title, x, y, kind, note, workload = null) => ({ id, title, x, y, kind, note, workload, healthy: healthy(workload) })
  const nginx = find((name) => name.includes('nginx-gm')); const mgr = find((name) => name.includes('mgr')); const gate = find((name) => name.includes('gate')); const login = find((name) => name.includes('login')); const notice = find((name) => name.includes('notice')); const social = find((name) => name.includes('social')); const zk = find((name) => name.includes('zookeeper') || name === 'zk')
  const result = [add('player', '플레이어 클라이언트', 34, 270, 'external', 'Gate / Notice 통해서만 통신')]
  if (nginx) result.push(add('nginx', nginx.name, 250, 74, 'management', '독립 GM 진입점', nginx))
  if (mgr) result.push(add('mgr', mgr.name, 460, 74, 'management', 'GM 백오피스', mgr))
  if (gate) result.push(add('gate', gate.name, 270, 280, 'public', 'WebSocket · 외부 진입점', gate))
  if (login) result.push(add('login', login.name, 505, 280, 'service', '로그인 검증', login))
  result.push(add('zk', zk?.name || 'ZooKeeper', 740, 280, 'dependency', 'Service 등록 / 활성 상태', zk || null))
  const addGameGroup = (items, prefix, startY) => items.forEach((item, index) => {
    const column = index % gameColumns.value
    const row = Math.floor(index / gameColumns.value)
    const node = add(`${prefix}-${index}`, item.name, gameStartX + column * nodeStepX, startY + row * nodeStepY, 'service', '상태 유지 게임 Service', item)
    node.statefulGroup = prefix
    node.instance = index + 1
    result.push(node)
  })
  addGameGroup(homeWorkloads.value, 'home', homeGroupY + statefulGroupPadding)
  addGameGroup(worldWorkloads.value, 'world', worldStartY.value)
  if (social) result.push(add('social', social.name, 740, Math.max(470, gameBottomY.value + 18), 'service', '소셜 Service · ZooKeeper 경유 탐색', social))
  if (notice) result.push(add('notice', notice.name, 270, 470, 'public', '로그 업로드 · 외부 진입점', notice))
  const known = new Set([nginx, mgr, gate, login, notice, social, zk, ...homeWorkloads.value, ...worldWorkloads.value].filter(Boolean).map((item) => item.name))
  workloads.value.filter((item) => !known.has(item.name)).forEach((item, index) => result.push(add(`extra-${index}`, item.name, 740 + (index % 1) * 230, 74 + Math.floor(index / 1) * 105, 'service', `${item.type} · Ready ${item.ready || '0/0'}`, item)))
  return result
})
const positions = computed(() => Object.fromEntries(nodes.value.map((item) => [item.id, item])))
const edges = computed(() => {
  const result = []; const has = (id) => Boolean(positions.value[id]); const add = (from, to, label, tone = 'default') => { if (has(from) && has(to)) result.push({ from, to, label, tone }) }
  add('player', 'gate', 'WebSocket 로그인', 'public'); add('player', 'notice', '로그 업로드', 'public'); add('gate', 'login', '로그인 요청', 'public'); add('login', 'zk', '활성 Service 조회'); add('zk', 'social', 'Service 등록'); add('mgr', 'zk', '기동/중지 관리', 'management')
  homeWorkloads.value.forEach((_, index) => { const id = `home-${index}`; add('zk', id, index === 0 ? 'Service 등록' : ''); add('gate', id, index === 0 ? '게임 세션' : ''); add(id, 'social', index === 0 ? '소셜 통신' : ''); add('mgr', id, index === 0 ? '게임 서버 관리' : '', 'management') })
  worldWorkloads.value.forEach((_, index) => { const id = `world-${index}`; add('zk', id, index === 0 ? 'Service 등록' : ''); add('gate', id, index === 0 ? '게임 세션' : ''); add(id, 'social', index === 0 ? '소셜 통신' : ''); add('mgr', id, index === 0 ? '게임 서버 관리' : '', 'management') })
  nodes.value.filter((item) => !['player', 'nginx', 'zk'].includes(item.id)).forEach((item) => add(item.id, 'nginx', 'GM 접속', 'management'))
  return result
})
function nodeWidth(node) { return node?.statefulGroup ? 190 : 174 }
function path(edge) { const from = positions.value[edge.from]; const to = positions.value[edge.to]; const x1 = from.x + nodeWidth(from); const y1 = from.y + 46; const x2 = to.x; const y2 = to.y + 46; const middle = Math.round((x1 + x2) / 2); return `M ${x1} ${y1} H ${middle} V ${y2} H ${x2}` }
function label(edge) { const from = positions.value[edge.from]; const to = positions.value[edge.to]; return { x: Math.round((from.x + nodeWidth(from) + to.x) / 2), y: Math.round((from.y + to.y) / 2) - 8 } }
function openDetail(node) { if (!node.workload) return; selectedWorkload.value = node.workload; selectedLog.value = null; activeDrawerTab.value = 'detail'; detailVisible.value = true }
function openLogs(target = null) { selectedLog.value = target; activeDrawerTab.value = 'logs' }
async function load() { if (!serviceId.value) return; loading.value = true; try { topology.value = await queryAssetServiceRuntimeTopology(serviceId.value) } finally { loading.value = false } }
onMounted(load)
</script>

<template>
  <div class="service-topology" v-loading="loading">
    <section class="topology-header"><div><el-button text :icon="ArrowLeft" @click="router.push('/containers/services')">Service 관리로 돌아가기</el-button><p class="eyebrow">SERVICE RESOURCE TOPOLOGY</p><h1>{{ topology.service?.name || 'Service Resource Topology' }}</h1><p><code>{{ topology.service?.serviceUid }}</code> · {{ topology.cluster?.name || 'Cluster 미연결' }} / {{ topology.namespace || '-' }}</p></div><el-button :icon="Refresh" @click="load">실행 상태 새로고침</el-button></section>
    <el-alert v-if="topology.refreshError" type="warning" :closable="false" show-icon :title="topology.refreshError" />
    <section class="topology-card"><header><div><h2>비즈니스 통신 경로</h2><p>녹색 ✓은 정상, 빨간색 !은 이상을 의미합니다. Home / World는 StatefulSet Service Group으로 표시되며 Instance 순서, Ready 상태와 상세 진입점을 유지합니다.</p></div><div class="legend"><span class="public"></span>외부 진입점 <span class="management"></span>GM 관리 <span class="service"></span>내부 Service <i class="legend-health ok">✓</i>정상 <i class="legend-health bad">!</i>이상</div></header><div class="topology-scroll"><div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${canvasHeight}px` }"><div v-for="group in statefulGroups" :key="group.key" class="stateful-group" :class="group.key" :style="{ left: `${group.x}px`, top: `${group.y}px`, width: `${group.width}px`, height: `${group.height}px` }"></div><svg :viewBox="`0 0 ${canvasWidth} ${canvasHeight}`" :width="canvasWidth" :height="canvasHeight"><defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#8ca1bf" /></marker></defs><g v-for="edge in edges" :key="`${edge.from}-${edge.to}`"><path :d="path(edge)" :class="['edge', edge.tone]" marker-end="url(#arrow)"/><text v-if="edge.label" :x="label(edge).x" :y="label(edge).y" text-anchor="middle">{{ edge.label }}</text></g></svg><button v-for="node in nodes" :key="node.id" class="topology-node" :class="[`node-${node.kind}`, { clickable: node.workload, 'node-stateful': node.statefulGroup }]" :style="{ left: `${node.x}px`, top: `${node.y}px` }" @click="openDetail(node)"><i v-if="node.workload" class="health-badge" :class="node.healthy ? 'is-ok' : 'is-bad'">{{ node.healthy ? '✓' : '!' }}</i><span v-if="node.statefulGroup" class="instance-chip">{{ node.statefulGroup.toUpperCase() }} #{{ node.instance }}</span><b>{{ node.title }}</b><small>{{ node.note }}</small><small v-if="node.workload">{{ node.workload.type }} · Ready {{ node.workload.ready || '0/0' }}</small></button></div></div></section>
    <section class="workload-card"><h2>연관 Workload</h2><el-table :data="workloads" size="small"><el-table-column prop="name" label="Workload"/><el-table-column prop="type" label="유형" width="120"/><el-table-column prop="ready" label="Ready" width="100"/><el-table-column label="Health" width="100"><template #default="{ row }"><el-tag :type="healthy(row) ? 'success' : 'danger'">{{ healthy(row) ? '✓ 정상' : '! 이상' }}</el-tag></template></el-table-column><el-table-column label="작업" width="110"><template #default="{ row }"><el-button link type="primary" @click="openDetail({ workload: row })">Service 상세</el-button></template></el-table-column></el-table></section>
    <el-drawer v-model="detailVisible" size="70%" :with-header="false" destroy-on-close>
      <div v-if="selectedWorkload" class="service-drawer-tabs"><el-button :type="activeDrawerTab === 'detail' ? 'primary' : 'default'" @click="activeDrawerTab = 'detail'">Service 상세</el-button><el-button :type="activeDrawerTab === 'logs' ? 'primary' : 'default'" @click="openLogs()">Service 로그</el-button><el-button :type="activeDrawerTab === 'monitor' ? 'primary' : 'default'" @click="activeDrawerTab = 'monitor'">Pod 모니터링</el-button></div>
      <ServiceWorkloadDetail v-if="activeDrawerTab === 'detail'" :service-id="serviceId" :workload-type="selectedWorkload.type" :workload-name="selectedWorkload.name" inline @close="detailVisible = false" @show-logs="openLogs" />
      <ServiceWorkloadLogs v-else-if="activeDrawerTab === 'logs'" :key="selectedLog?.podName || 'default'" :service-id="serviceId" :workload-type="selectedWorkload.type" :workload-name="selectedWorkload.name" :pod-name="selectedLog?.podName || ''" inline @close="detailVisible = false" />
      <ServicePodMonitor v-else :service-id="serviceId" :workload-type="selectedWorkload.type" :workload-name="selectedWorkload.name" />
    </el-drawer>
  </div>
</template>

<style scoped>
.service-topology{padding:24px;background:#f4f7fc;min-height:100%}.topology-header,.topology-card,.workload-card{background:#fff;border:1px solid #e0e8f5;border-radius:18px}.topology-header{display:flex;justify-content:space-between;align-items:center;padding:22px 28px;margin-bottom:16px;background:linear-gradient(120deg,#fff,#eef4ff)}.eyebrow{font-size:12px;letter-spacing:.08em;color:#3970df;font-weight:800;margin:10px 0 4px}.topology-header h1{margin:0;color:#102b54}.topology-header p{margin:8px 0 0;color:#7485a0}.topology-header code{color:#4168b5}.topology-card{overflow:hidden;margin-top:16px}.topology-card>header{display:flex;justify-content:space-between;align-items:center;padding:20px 24px;border-bottom:1px solid #e7edf7}.topology-card h2,.workload-card h2{margin:0;color:#142e57;font-size:18px}.topology-card p{margin:7px 0 0;color:#7687a1;font-size:13px}.legend{display:flex;gap:8px;align-items:center;flex-wrap:wrap;color:#7586a0;font-size:12px}.legend span{width:10px;height:10px;border-radius:50%;margin-left:8px}.legend .public{background:#2585ee}.legend .management{background:#8856e9}.legend .service{background:#18ad82}.legend-health{width:18px;height:18px;border-radius:50%;display:grid;place-items:center;color:#fff;font-style:normal;font-weight:800}.legend-health.ok{background:#16b68a}.legend-health.bad{background:#ef5259}.canvas{position:relative;min-width:1200px;height:600px;background-image:radial-gradient(#d7e3f4 1px,transparent 1px);background-size:18px 18px}.canvas>svg{position:absolute;inset:0;width:1200px;height:600px}.edge{fill:none;stroke:#8ca1bf;stroke-width:1.8}.edge.public{stroke:#2585ee;stroke-width:2.6}.edge.management{stroke:#8856e9;stroke-dasharray:6 4}.canvas text{font-size:11px;fill:#70829e;paint-order:stroke;stroke:#fff;stroke-width:4px}.topology-node{position:absolute;width:174px;min-height:92px;padding:12px 14px;border:2px solid #d5e0f0;border-radius:14px;background:#fff;box-shadow:0 8px 18px rgba(37,62,113,.1);text-align:left}.topology-node.clickable{cursor:pointer}.topology-node.clickable:hover{transform:translateY(-2px);box-shadow:0 10px 22px rgba(37,62,113,.18)}.topology-node b,.topology-node small{display:block}.topology-node b{padding-right:28px;color:#152e56;font-size:14px}.topology-node small{margin-top:5px;color:#7587a4;font-size:11px}.health-badge{position:absolute;right:12px;top:12px;width:19px;height:19px;border-radius:50%;display:grid;place-items:center;color:#fff;font-style:normal;font-size:13px;font-weight:800;line-height:1}.health-badge.is-ok{background:#16b68a}.health-badge.is-bad{background:#ef5259}.node-public{border-color:#55a1f2;background:#f3f9ff}.node-management{border-color:#a27cf0;background:#faf7ff}.node-dependency{border-color:#f0a640;background:#fffaf0}.node-service{border-color:#50caa0;background:#f3fffa}.node-external{border-color:#778eb0;background:#f7f9fd}.workload-card{margin-top:16px;padding:20px}.workload-card h2{margin-bottom:16px}.service-drawer-tabs{position:sticky;top:0;z-index:3;display:flex;gap:10px;padding:14px 24px 0;background:#fff;border-bottom:1px solid #e6edf7}@media(max-width:1200px){.topology-card{overflow:auto}.topology-card>header{align-items:flex-start;gap:12px;flex-direction:column}}
</style>

<style scoped>
.topology-scroll{overflow:auto;padding-bottom:2px}
.canvas{min-width:1200px;zoom:1}
.canvas>svg{width:100%;height:100%}
.topology-card{overflow:hidden}
.topology-node{transition:transform .16s ease,box-shadow .16s ease}
.topology-node b,.topology-node small{overflow-wrap:anywhere;word-break:break-word;line-height:1.35}
.stateful-group{position:absolute;z-index:0;padding:10px 12px;border:1px dashed #9acdbd;border-radius:18px;background:rgba(239,255,248,.72)}
.stateful-group.world{border-color:#9dbce9;background:rgba(241,247,255,.76)}
.stateful-group-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}
.stateful-group-head b,.stateful-group-head small{display:block}.stateful-group-head b{color:#17694f;font-size:13px}.stateful-group.world .stateful-group-head b{color:#386bb4}.stateful-group-head small{margin-top:2px;color:#6e8796;font-size:11px}
.canvas>svg{z-index:1}.topology-node{z-index:2}
.node-stateful{width:190px;min-height:104px;padding-top:28px;border-color:#35bb8b;background:#f5fffb}.node-stateful .instance-chip{position:absolute;top:9px;left:12px;padding:2px 6px;border-radius:999px;background:#dff7ed;color:#17825d;font-size:10px;font-weight:800;letter-spacing:.04em}.node-stateful b{font-size:15px}.node-stateful small{margin-top:6px}
</style>
