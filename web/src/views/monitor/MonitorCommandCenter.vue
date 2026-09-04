<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { FullScreen, Refresh, RefreshLeft } from '@element-plus/icons-vue'
import { queryMonitorCommandCenter } from '../../api/monitor'
import { currentLocale } from '../../utils/i18n-runtime'
import { mt } from '../../utils/monitor-i18n'

const screenRef = ref(null)
const globeRef = ref(null)
const loading = ref(false)
const autoRefresh = ref(true)
const now = ref(new Date())
const data = ref({ overview: {}, assetSummary: {}, resourceComposition: [], regions: [], topRules: [], recentAlerts: [], hotHosts: [] })
const globe = { yaw: -0.78, pitch: -0.12, dragging: false, lastX: 0, lastY: 0, canvas: null, ctx: null }
// Simplified continent outlines keep the globe fully local while making the
// asset-network view recognizable instead of looking like an abstract dot ball.
const continentShapes = [
  [[-168,72],[-145,70],[-128,58],[-125,49],[-117,35],[-106,31],[-97,23],[-82,25],[-75,39],[-66,47],[-58,54],[-72,59],[-89,62],[-106,72],[-132,76]],
  [[-81,12],[-72,8],[-67,-4],[-61,-13],[-58,-26],[-61,-39],[-68,-54],[-74,-50],[-78,-30],[-80,-10]],
  [[-10,36],[5,44],[18,53],[38,58],[55,56],[72,61],[92,68],[125,59],[143,51],[151,43],[137,33],[123,25],[108,21],[97,8],[83,8],[74,22],[57,27],[43,38],[27,38],[14,33],[2,36]],
  [[-17,35],[-2,37],[15,32],[26,22],[31,11],[28,-3],[22,-16],[19,-34],[12,-35],[5,-23],[-2,-4],[-10,12]],
  [[112,-11],[128,-12],[143,-19],[151,-31],[145,-41],[132,-39],[120,-31]],
  [[-57,81],[-39,83],[-22,76],[-31,64],[-48,61],[-58,69]],
  [[129,34],[141,40],[146,35],[138,31]], [[95,5],[112,3],[118,-5],[108,-8],[98,-4]]
]
const networkSites = [[-122,37],[-74,41],[-46,-23],[-3,52],[31,30],[77,28],[116,40],[139,35],[121,-31]]
let animationFrame
let clockTimer
let refreshTimer

const asset = computed(() => data.value.assetSummary || {})
const overview = computed(() => data.value.overview || {})
const alertTrend = computed(() => overview.value.trend || [])
const trendMax = computed(() => Math.max(1, ...alertTrend.value.flatMap((item) => [Number(item.triggered || 0), Number(item.recovered || 0)])))
const onlineRate = computed(() => Number(asset.value.coverage || 0))

function number(value) { return Number(value || 0).toLocaleString() }
function percent(value) { return `${Number(value || 0).toFixed(1)}%` }
function timeText(value = now.value) { return new Date(value).toLocaleTimeString(currentLocale.value, { hour12: false }) }
function dateText(value = now.value) { return new Date(value).toLocaleDateString(currentLocale.value, { year: 'numeric', month: '2-digit', day: '2-digit' }) }
function severityType(value) { return ['P0', 'P1'].includes(String(value || '').toUpperCase()) ? 'danger' : String(value || '').toUpperCase() === 'P2' ? 'warning' : 'info' }

async function loadData() {
  loading.value = true
  try { data.value = await queryMonitorCommandCenter() || data.value } finally { loading.value = false }
}

function project(lon, lat, radius, cx, cy) {
  const lambda = lon + globe.yaw
  const phi = lat
  const x = Math.cos(phi) * Math.sin(lambda)
  const y = Math.sin(phi) * Math.cos(globe.pitch) - Math.cos(phi) * Math.cos(lambda) * Math.sin(globe.pitch)
  const z = Math.sin(phi) * Math.sin(globe.pitch) + Math.cos(phi) * Math.cos(lambda) * Math.cos(globe.pitch)
  return { x: cx + x * radius, y: cy - y * radius, z }
}

function pointInPolygon(lon, lat, polygon) {
  let inside = false
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const [xi, yi] = polygon[i]; const [xj, yj] = polygon[j]
    if ((yi > lat) !== (yj > lat) && lon < (xj - xi) * (lat - yi) / (yj - yi) + xi) inside = !inside
  }
  return inside
}
function continentDensity(lon, lat) { return continentShapes.some((shape) => pointInPolygon(lon, lat, shape)) }
const continentPoints = []
for (let lat = -78; lat <= 78; lat += 2.4) {
  for (let lon = -180; lon <= 180; lon += 2.4) {
    if (continentDensity(lon, lat)) continentPoints.push([lon, lat])
  }
}

function drawGlobe() {
  const canvas = globe.canvas
  const ctx = globe.ctx
  if (!canvas || !ctx) return
  const ratio = window.devicePixelRatio || 1
  const width = canvas.clientWidth
  const height = canvas.clientHeight
  if (canvas.width !== Math.round(width * ratio) || canvas.height !== Math.round(height * ratio)) {
    canvas.width = Math.round(width * ratio); canvas.height = Math.round(height * ratio)
  }
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0)
  ctx.clearRect(0, 0, width, height)
  const radius = Math.min(width, height) * 0.33
  const cx = width * 0.5
  const cy = height * 0.52
  const atmosphere = ctx.createRadialGradient(cx - radius * .2, cy - radius * .25, radius * .18, cx, cy, radius * 1.3)
  atmosphere.addColorStop(0, 'rgba(64, 218, 255, .32)')
  atmosphere.addColorStop(.62, 'rgba(8, 75, 150, .28)')
  atmosphere.addColorStop(1, 'rgba(0, 194, 255, 0)')
  ctx.fillStyle = atmosphere; ctx.beginPath(); ctx.arc(cx, cy, radius * 1.3, 0, Math.PI * 2); ctx.fill()
  ctx.save(); ctx.beginPath(); ctx.arc(cx, cy, radius, 0, Math.PI * 2); ctx.clip()
  const surface = ctx.createRadialGradient(cx - radius * .25, cy - radius * .3, radius * .1, cx, cy, radius)
  surface.addColorStop(0, '#174f89'); surface.addColorStop(.55, '#071b3d'); surface.addColorStop(1, '#020a1c')
  ctx.fillStyle = surface; ctx.fillRect(cx - radius, cy - radius, radius * 2, radius * 2)
  ctx.lineWidth = 1
  for (let lat = -60; lat <= 60; lat += 20) {
    ctx.beginPath(); let visible = false
    for (let lon = -180; lon <= 180; lon += 3) {
      const point = project(lon * Math.PI / 180, lat * Math.PI / 180, radius, cx, cy)
      if (point.z > -0.05) { visible ? ctx.lineTo(point.x, point.y) : ctx.moveTo(point.x, point.y); visible = true }
    }
    ctx.strokeStyle = 'rgba(94, 206, 255, .16)'; ctx.stroke()
  }
  for (let lon = -160; lon <= 180; lon += 20) {
    ctx.beginPath(); let visible = false
    for (let lat = -88; lat <= 88; lat += 3) {
      const point = project(lon * Math.PI / 180, lat * Math.PI / 180, radius, cx, cy)
      if (point.z > -0.05) { visible ? ctx.lineTo(point.x, point.y) : ctx.moveTo(point.x, point.y); visible = true }
    }
    ctx.strokeStyle = 'rgba(94, 206, 255, .12)'; ctx.stroke()
  }
  for (const [lon, lat] of continentPoints) {
    const point = project(lon * Math.PI / 180, lat * Math.PI / 180, radius, cx, cy)
    if (point.z <= 0) continue
    const alpha = .12 + point.z * .78
    ctx.fillStyle = `rgba(61, 228, 255, ${alpha})`
    const size = point.z > .6 ? 2.05 : 1.2
    ctx.fillRect(point.x, point.y, size, size)
  }
  ctx.restore()
  for (const [lon, lat] of networkSites) {
    const point = project(lon * Math.PI / 180, lat * Math.PI / 180, radius, cx, cy)
    if (point.z <= .1) continue
    ctx.strokeStyle = `rgba(255, 207, 89, ${point.z})`; ctx.lineWidth = 1
    ctx.beginPath(); ctx.arc(point.x, point.y, 3 + point.z * 2, 0, Math.PI * 2); ctx.stroke()
    ctx.fillStyle = '#ffd45e'; ctx.beginPath(); ctx.arc(point.x, point.y, 1.8, 0, Math.PI * 2); ctx.fill()
  }
  ctx.strokeStyle = 'rgba(70, 214, 255, .74)'; ctx.lineWidth = 2; ctx.beginPath(); ctx.arc(cx, cy, radius, 0, Math.PI * 2); ctx.stroke()
  ctx.strokeStyle = 'rgba(74, 204, 255, .18)'; ctx.lineWidth = 1
  ctx.beginPath(); ctx.ellipse(cx, cy, radius * 1.27, radius * .47, -.18, 0, Math.PI * 2); ctx.stroke()
  ctx.beginPath(); ctx.ellipse(cx, cy, radius * 1.08, radius * .88, .7, 0, Math.PI * 2); ctx.stroke()
  ctx.fillStyle = '#e9c95c'; ctx.beginPath(); ctx.arc(cx + radius * .98, cy - radius * .26, 4, 0, Math.PI * 2); ctx.fill()
}

function animateGlobe() {
  if (!globe.dragging) globe.yaw += .0018
  drawGlobe()
  animationFrame = requestAnimationFrame(animateGlobe)
}
function resetGlobe() { globe.yaw = -0.78; globe.pitch = -0.12; drawGlobe() }
function onPointerDown(event) { globe.dragging = true; globe.lastX = event.clientX; globe.lastY = event.clientY; globe.canvas?.setPointerCapture?.(event.pointerId) }
function onPointerMove(event) {
  if (!globe.dragging) return
  globe.yaw += (event.clientX - globe.lastX) * .008
  globe.pitch = Math.max(-.9, Math.min(.9, globe.pitch + (event.clientY - globe.lastY) * .006))
  globe.lastX = event.clientX; globe.lastY = event.clientY; drawGlobe()
}
function onPointerUp() { globe.dragging = false }
function onWheel(event) { event.preventDefault(); globe.pitch = Math.max(-.9, Math.min(.9, globe.pitch - event.deltaY * .0008)); drawGlobe() }
async function toggleFullscreen() {
  if (document.fullscreenElement) await document.exitFullscreen()
  else await screenRef.value?.requestFullscreen?.()
}
function syncTimers() {
  window.clearInterval(refreshTimer)
  refreshTimer = autoRefresh.value ? window.setInterval(loadData, 30000) : undefined
}

onMounted(async () => {
  await nextTick()
  globe.canvas = globeRef.value; globe.ctx = globe.canvas?.getContext('2d')
  globe.canvas?.addEventListener('wheel', onWheel, { passive: false })
  window.addEventListener('resize', drawGlobe)
  animateGlobe(); clockTimer = window.setInterval(() => { now.value = new Date() }, 1000); syncTimers(); await loadData()
})
onBeforeUnmount(() => {
  cancelAnimationFrame(animationFrame); window.clearInterval(clockTimer); window.clearInterval(refreshTimer)
  globe.canvas?.removeEventListener('wheel', onWheel); window.removeEventListener('resize', drawGlobe)
})
</script>

<template>
  <div ref="screenRef" class="command-center" :class="{ loading }">
    <header class="cc-header">
      <div class="cc-system"><i /> 시스템 온라인 <span>데이터 새로고침: {{ timeText(data.refreshedAt || now) }}</span></div>
      <div class="cc-title"><small>{{ mt('commandCenterEyebrow') }}</small><strong>지능형 운영 Cockpit</strong></div>
      <div class="cc-tools"><b>{{ timeText() }}</b><span>{{ dateText() }}</span><div class="cc-auto" :class="{ enabled: autoRefresh }"><i /><span>자동 새로고침</span><b>30초</b><el-switch v-model="autoRefresh" size="small" @change="syncTimers" /></div><el-button circle :icon="Refresh" :loading="loading" @click="loadData" /><el-button circle :icon="FullScreen" @click="toggleFullscreen" /></div>
    </header>

    <main class="cc-layout">
      <section class="cc-column left-column">
        <article class="cc-panel alerts-panel"><div class="panel-heading"><span>▣ 실시간 Alert</span><router-link to="/monitor/alert-events">전체 보기 {{ number(overview.firingCount) }}</router-link></div><div class="alert-list"><div v-for="item in data.recentAlerts" :key="item.id" class="alert-item"><el-tag size="small" :type="severityType(item.severity)">{{ item.severity || 'P3' }}</el-tag><div><b>{{ item.ruleName || '이름 없는 Alert' }}</b><p>{{ item.summary || item.metric || '처리 대기' }}</p></div></div><div v-if="!data.recentAlerts?.length" class="panel-empty">◇ 현재 활성 Alert가 없습니다</div></div></article>
        <article class="cc-panel trend-panel"><div class="panel-heading"><span>▣ 오늘 Alert 추세</span><em>발생 / 복구</em></div><div class="mini-bars"><div v-for="item in alertTrend" :key="item.date" class="mini-bar"><div class="bar-stack"><i :style="{ height: `${Math.max(3, Number(item.triggered || 0) / trendMax * 100)}%` }" /><b :style="{ height: `${Math.max(3, Number(item.recovered || 0) / trendMax * 100)}%` }" /></div><small>{{ item.date }}</small></div><div v-if="!alertTrend.length" class="panel-empty">◇ 오늘 추세 데이터가 없습니다</div></div></article>
        <article class="cc-panel rule-panel"><div class="panel-heading"><span>▣ Alert Rule TOP5</span><em>활성 Event</em></div><div class="rank-list"><div v-for="(item, index) in data.topRules" :key="item.name"><b>0{{ index + 1 }}</b><span>{{ item.name }}</span><strong>{{ number(item.count) }}</strong></div><div v-if="!data.topRules?.length" class="panel-empty">◇ 순위 데이터가 없습니다</div></div></article>
      </section>

      <section class="cc-center">
        <div class="metric-strip"><div class="metric-card"><small>자산 총량</small><strong>{{ number(asset.total) }}</strong><span>관리 대상 자산</span></div><div class="metric-card danger"><small>활성 Alert</small><strong>{{ number(overview.firingCount) }}</strong><span>처리 대기 Event</span></div><div class="metric-card"><small>온라인 Host</small><strong>{{ number(asset.onlineHosts) }} / {{ number(asset.hosts) }}</strong><span>Host 생존율</span></div><div class="metric-card green"><small>모니터링 커버리지</small><strong>{{ percent(onlineRate) }}</strong><span>Host 온라인 상태 기준</span></div><div class="metric-card amber"><small>K8s Node</small><strong>{{ number(asset.clusters) }}</strong><span>연동된 Cluster</span></div></div>
        <article class="globe-panel"><div class="globe-heading"><div><small>{{ mt('resourceSituation') }}</small><h2>리소스 공간 현황</h2><p>지구본을 드래그해 리소스 네트워크를 확인하십시오.</p></div><button type="button" @click="resetGlobe"><el-icon><RefreshLeft /></el-icon> 시점 초기화</button></div><canvas ref="globeRef" class="globe-canvas" @pointerdown="onPointerDown" @pointermove="onPointerMove" @pointerup="onPointerUp" @pointerleave="onPointerUp" /><div class="globe-stats left"><span>{{ mt('physicalHost') }}</span><b>{{ number(asset.hosts) }}</b><small>물리 / Cloud Host</small><span>{{ mt('monitoring') }}</span><b>{{ percent(onlineRate) }}</b><small>온라인 커버리지</small></div><div class="globe-stats right"><span>{{ mt('activeAlert') }}</span><b class="danger-text">{{ number(overview.firingCount) }}</b><small>활성 Alert</small><span>{{ mt('authRate') }}</span><b>{{ percent(onlineRate) }}</b><small>자산 인증률</small></div><footer><b>{{ number(asset.total) }}</b><span>관리 자산</span><small>● Data Center Node　● 자산 규모　지구본을 드래그해 회전</small></footer></article>
        <div class="hot-row"><article v-for="title in ['CPU 핫스팟', '메모리 핫스팟', '디스크 핫스팟']" :key="title" class="cc-panel hot-panel"><div class="panel-heading"><span>▣ {{ title }}</span><em>{{ mt('topFive') }}</em></div><div v-if="data.hotHosts?.length" class="hot-host"><div v-for="host in data.hotHosts.slice(0, 3)" :key="host.name"><span>{{ host.name }}</span><b :class="host.aliveStatus === 1 ? 'ok-text' : 'danger-text'">{{ host.aliveStatus === 1 ? '온라인' : '점검 필요' }}</b></div></div><div v-else class="panel-empty">◇ 모니터링 데이터가 없습니다</div></article></div>
      </section>

      <section class="cc-column right-column">
        <article class="cc-panel composition-panel"><div class="panel-heading"><span>▣ 자산 브랜드 구성</span><em>{{ mt('topCount', { count: data.resourceComposition?.length || 0 }) }}</em></div><div class="composition-list"><div v-for="item in data.resourceComposition" :key="item.name"><span>{{ item.name }}</span><i><b :style="{ width: `${asset.total ? Number(item.count || 0) / asset.total * 100 : 0}%` }" /></i><strong>{{ number(item.count) }}</strong></div></div></article>
        <article class="cc-panel resource-panel"><div class="panel-heading"><span>▣ 자산 리소스 개요</span><em>총 {{ number(asset.total) }}건</em></div><div class="resource-total"><div v-for="item in data.resourceComposition" :key="item.name"><span>{{ item.name }}</span><b>{{ number(item.count) }}</b></div></div><div class="coverage-ring"><strong>{{ percent(onlineRate) }}</strong><span>온라인 커버리지</span></div></article>
        <article class="cc-panel region-panel"><div class="panel-heading"><span>▣ Data Center 자산 분포</span><em>{{ data.regions?.length || 0 }}개 지역</em></div><div class="region-list"><div v-for="item in data.regions" :key="item.name"><i>◇</i><span>{{ item.name }}</span><b>{{ number(item.count) }}</b></div><div v-if="!data.regions?.length" class="panel-empty">◇ 지역 분포 데이터가 없습니다</div></div></article>
        <div class="deadline"><span>◷ 대응 대기 Alert <b>{{ number(overview.unclaimedCount) }}</b></span><span>● 높은 우선순위 <b>{{ number(overview.criticalCount) }}</b></span></div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.command-center{--cyan:#35d8ff;--line:rgba(49,194,255,.3);--panel:rgba(3,28,52,.88);min-height:calc(100vh - 80px);padding:10px;background:#020d1d radial-gradient(circle at 50% 42%,#0a2b50 0,transparent 31%),linear-gradient(135deg,#020b18,#04192d 52%,#02111f);color:#d9f6ff;font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif}.cc-header{position:relative;display:flex;align-items:center;justify-content:space-between;min-height:58px;border-top:1px solid #1b6a91;border-bottom:1px solid #1b6a91;background:linear-gradient(90deg,transparent,#062844 25%,#062844 75%,transparent)}.cc-header:before,.cc-header:after{position:absolute;top:0;width:22%;height:100%;border-bottom:2px solid var(--cyan);content:""}.cc-header:before{left:0}.cc-header:after{right:0}.cc-system,.cc-tools{z-index:1;display:flex;align-items:center;gap:10px;width:31%;padding:0 12px;color:#8eeaff;font-size:13px}.cc-system i{width:8px;height:8px;border-radius:50%;background:#36eba7;box-shadow:0 0 12px #36eba7}.cc-system span,.cc-tools span{color:#6390a8;font-size:11px}.cc-title{z-index:1;text-align:center}.cc-title small{display:block;color:#35c5df;font-size:8px;letter-spacing:.16em}.cc-title strong{display:block;color:#f2fdff;font-size:25px;letter-spacing:.08em;text-shadow:0 0 12px #54b7d7}.cc-tools{justify-content:flex-end}.cc-tools b{font-family:monospace;font-size:21px}.cc-tools :deep(.el-button){width:29px;height:29px;border-color:#236080;background:#072640;color:#65d9ff}.cc-layout{display:grid;grid-template-columns:minmax(250px,25%) minmax(530px,1fr) minmax(250px,25%);gap:10px;margin-top:10px}.cc-column,.cc-center{display:flex;flex-direction:column;gap:10px;min-width:0}.cc-panel,.globe-panel{position:relative;border:1px solid var(--line);background:var(--panel);box-shadow:inset 0 0 25px rgba(10,137,198,.08),0 0 20px rgba(0,0,0,.18);overflow:hidden}.cc-panel:before,.globe-panel:before{position:absolute;top:0;left:0;width:68px;height:2px;background:var(--cyan);box-shadow:0 0 12px var(--cyan);content:""}.panel-heading{display:flex;justify-content:space-between;align-items:center;min-height:42px;padding:0 10px;border-bottom:1px solid rgba(59,187,240,.25);color:#9eeaff;font-weight:700;font-size:14px}.panel-heading a,.panel-heading em{color:#5898b9;font-style:normal;font-size:11px;text-decoration:none}.panel-empty{display:grid;place-items:center;min-height:94px;color:#4e7791;font-size:12px}.alerts-panel{height:34vh;min-height:230px}.alert-list{padding:8px}.alert-item{display:flex;gap:8px;padding:8px 3px;border-bottom:1px dashed rgba(74,155,192,.18)}.alert-item b{display:block;overflow:hidden;max-width:220px;color:#cceeff;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.alert-item p{overflow:hidden;margin:4px 0 0;color:#6290a9;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.trend-panel{height:25vh;min-height:180px}.mini-bars{display:flex;align-items:flex-end;gap:7px;height:calc(100% - 50px);padding:12px}.mini-bar{display:flex;flex:1;flex-direction:column;justify-content:flex-end;min-width:0;height:100%;gap:5px;text-align:center}.bar-stack{display:flex;align-items:flex-end;justify-content:center;gap:2px;height:100%}.bar-stack i,.bar-stack b{display:block;width:42%;min-height:2px;background:#32c8f3}.bar-stack b{background:#eac45b}.mini-bar small{color:#527c96;font-size:9px}.rule-panel{flex:1;min-height:176px}.rank-list{padding:8px}.rank-list>div{display:grid;grid-template-columns:32px 1fr 28px;gap:7px;align-items:center;padding:7px 2px;border-bottom:1px dashed rgba(73,151,191,.18);font-size:12px}.rank-list b{color:#f2c95e}.rank-list span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.rank-list strong{color:#72ddfa;text-align:right}.metric-strip{display:grid;grid-template-columns:repeat(5,1fr);gap:8px}.metric-card{min-height:94px;padding:15px 12px;border:1px solid #277ca1;border-bottom:3px solid #31c8e9;background:linear-gradient(145deg,#082b4b,#04182e)}.metric-card small,.metric-card span{display:block;color:#6aa0b7;font-size:10px}.metric-card strong{display:block;margin:7px 0 5px;color:#f2fdff;font-family:monospace;font-size:22px}.metric-card.danger{border-bottom-color:#ff5c69}.metric-card.green{border-bottom-color:#32d5a0}.metric-card.amber{border-bottom-color:#edc15c}.globe-panel{height:calc(100vh - 280px);min-height:480px;background-image:linear-gradient(rgba(50,152,208,.08) 1px,transparent 1px),linear-gradient(90deg,rgba(50,152,208,.08) 1px,transparent 1px),radial-gradient(circle at 50% 45%,rgba(12,81,143,.3),transparent 45%);background-size:18px 18px,18px 18px,auto}.globe-heading{position:absolute;z-index:2;top:13px;left:14px;display:flex;justify-content:space-between;width:calc(100% - 28px)}.globe-heading small{color:#3daec7;font-size:9px}.globe-heading h2{margin:2px 0;color:#d9f8ff;font-size:18px}.globe-heading p{margin:0;color:#6799b0;font-size:10px}.globe-heading button{height:30px;border:1px solid #236888;background:#082943;color:#87dff7;cursor:pointer}.globe-canvas{width:100%;height:100%;touch-action:none;cursor:grab}.globe-canvas:active{cursor:grabbing}.globe-stats{position:absolute;top:38%;display:grid;gap:3px;width:120px}.globe-stats.left{left:16px}.globe-stats.right{right:16px;text-align:right}.globe-stats span{margin-top:10px;color:#5d9ab6;font-size:9px}.globe-stats b{color:#ddfaff;font-family:monospace;font-size:21px}.globe-stats small{color:#56839d;font-size:10px}.globe-panel footer{position:absolute;bottom:11px;left:0;right:0;display:flex;flex-direction:column;align-items:center;color:#7cbcd0;font-size:10px}.globe-panel footer b{color:#f4ffff;font-family:monospace;font-size:22px}.globe-panel footer small{position:absolute;right:18px;bottom:0;color:#6aa4bb}.hot-row{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}.hot-panel{min-height:133px}.hot-host{padding:7px 10px}.hot-host div{display:flex;justify-content:space-between;padding:5px 0;border-bottom:1px dashed rgba(86,164,194,.17);font-size:11px}.hot-host span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.hot-host b{font-weight:500}.composition-panel{height:34vh;min-height:230px}.composition-list{padding:17px 12px}.composition-list>div{display:grid;grid-template-columns:88px 1fr 26px;gap:7px;align-items:center;margin-bottom:15px;color:#8db9cc;font-size:12px}.composition-list i{height:5px;background:#0b3854}.composition-list i b{display:block;height:100%;min-width:3px;background:linear-gradient(90deg,#21bce5,#52e1ef);box-shadow:0 0 8px #37dfff}.composition-list strong{color:#dffbff;text-align:right}.resource-panel{height:25vh;min-height:180px}.resource-total{padding:13px 116px 6px 13px}.resource-total div{display:flex;justify-content:space-between;margin-bottom:9px;color:#86b3c6;font-size:12px}.resource-total b{color:#e5fcff}.coverage-ring{position:absolute;right:20px;bottom:27px;display:grid;place-items:center;width:86px;height:86px;border:8px solid #0e4565;border-top-color:#36d5f2;border-radius:50%;box-shadow:0 0 15px rgba(51,214,242,.28)}.coverage-ring strong{color:#e4fcff;font-size:17px}.coverage-ring span{color:#638ea3;font-size:9px}.region-panel{flex:1;min-height:176px}.region-list{padding:10px}.region-list>div{display:grid;grid-template-columns:20px 1fr 30px;gap:6px;padding:8px 0;border-bottom:1px dashed rgba(73,151,191,.18);color:#82b4c7;font-size:12px}.region-list i{color:#3ad8f4;font-style:normal}.region-list b{color:#d7fbff;text-align:right}.deadline{display:grid;grid-template-columns:1fr 1fr;border:1px solid var(--line);background:#06223b;color:#94c4d6;font-size:11px}.deadline span{padding:9px}.deadline span+span{border-left:1px solid var(--line)}.deadline b{float:right;color:#f3ffff}.danger-text{color:#ff6c75!important}.ok-text{color:#43e0ae!important}.cc-auto{position:fixed;right:17px;bottom:13px;display:flex;align-items:center;gap:6px;color:#6f99af;font-size:11px}.command-center:fullscreen{min-height:100vh;padding:10px}.command-center:fullscreen .globe-panel{height:calc(100vh - 274px)}@media(max-width:1320px){.cc-layout{grid-template-columns:220px minmax(470px,1fr) 220px}.cc-title strong{font-size:20px}.globe-stats{display:none}}@media(max-width:1050px){.cc-layout{grid-template-columns:1fr}.left-column,.right-column{display:grid;grid-template-columns:repeat(3,1fr)}.alerts-panel,.trend-panel,.composition-panel,.resource-panel{height:220px}.globe-panel{height:550px}.cc-header{flex-wrap:wrap}.cc-system,.cc-tools{width:33%}}@media(max-width:720px){.command-center{padding:5px}.cc-title strong{font-size:16px}.cc-system span,.cc-tools span{display:none}.cc-system,.cc-tools{width:25%}.metric-strip,.hot-row,.left-column,.right-column{grid-template-columns:1fr 1fr}.metric-card{min-height:74px;padding:9px}.metric-card strong{font-size:16px}.globe-panel{height:430px;min-height:430px}.globe-panel footer small{display:none}.cc-tools b{font-size:14px}}
/* Keep the complete cockpit visible in the ordinary layout. Fullscreen uses the
   available viewport, while normal pages retain natural document scrolling. */
.command-center { position: relative; min-height: 0; padding: 8px; }
.cc-layout { align-items: start; }
.alerts-panel, .composition-panel { height: min(28vh, 250px); min-height: 210px; }
.trend-panel, .resource-panel { height: min(20vh, 175px); min-height: 155px; }
.rule-panel, .region-panel { flex: 0 0 160px; min-height: 160px; }
.globe-panel { height: clamp(405px, 48vh, 510px); min-height: 0; }
.hot-panel { min-height: 116px; }
.cc-auto { position: static; z-index: 4; }
.command-center:fullscreen { min-height: 100vh; padding: 10px; }
.command-center:fullscreen .globe-panel { height: clamp(470px, 56vh, 680px); }
@media (max-width: 1050px) {
  .alerts-panel, .trend-panel, .composition-panel, .resource-panel { height: 200px; }
  .rule-panel, .region-panel { flex-basis: 170px; }
  .globe-panel { height: 470px; }
  .cc-auto { position: static; }
}

/* Fill the available canvas instead of leaving a dead area under the panels. */
.command-center { display: flex; flex-direction: column; min-height: calc(100vh - 120px); }
.cc-layout { flex: 1; min-height: 720px; align-items: stretch; }
.cc-column, .cc-center { min-height: 0; }
.rule-panel, .region-panel { flex: 1 1 160px; }
.globe-panel { flex: 1 1 405px; height: auto; }
.command-center:fullscreen { height: 100vh; min-height: 0; overflow: hidden; }
.command-center:fullscreen .cc-layout { min-height: 0; }
.command-center:fullscreen .globe-panel { flex: 1 1 auto; height: auto; }
.cc-auto { display: inline-flex; align-items: center; gap: 7px; min-height: 32px; padding: 0 8px 0 10px; border: 1px solid rgba(91, 175, 215, .42); border-radius: 4px; background: rgba(2, 31, 57, .92); box-shadow: 0 0 13px rgba(27, 177, 235, .18), inset 0 0 12px rgba(43, 180, 239, .08); color: #8bb9cd; font-size: 11px; letter-spacing: .04em; }
.cc-auto i { width: 7px; height: 7px; border-radius: 50%; background: #506f80; }
.cc-auto b { padding: 2px 5px; border-radius: 2px; background: rgba(93, 135, 155, .18); color: #c2e6f3; font-family: monospace; font-size: 11px; }
.cc-auto.enabled { border-color: rgba(46, 221, 185, .74); color: #c8fff0; box-shadow: 0 0 16px rgba(32, 232, 182, .28), inset 0 0 14px rgba(28, 211, 176, .12); }
.cc-auto.enabled i { background: #32e5ae; box-shadow: 0 0 10px #32e5ae; animation: cc-refresh-pulse 1.8s ease-in-out infinite; }
.cc-auto.enabled b { background: rgba(39, 220, 166, .18); color: #80ffda; }
.cc-auto :deep(.el-switch) { --el-switch-on-color: #20cda0; --el-switch-off-color: #506f80; }
@keyframes cc-refresh-pulse { 50% { opacity: .4; transform: scale(.72); } }
@media (max-width: 1050px) {
  .command-center { min-height: 0; }
  .cc-layout { min-height: 0; }
}
</style>
