<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Connection, EditPen, Monitor } from '@element-plus/icons-vue'
import { assetDatabaseInfo, assetHostInfo, queryAssetChangeLogs } from '../../api/asset'
import { queryK8sClusterInfo } from '../../api/k8s'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import HostMetrics from './HostMetrics.vue'
import DatabaseMetrics from './DatabaseMetrics.vue'

const props = defineProps({ resourceType: { type: String, required: true } })
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const asset = ref({})
const changes = ref([])
const { environmentName } = useEnvironmentOptions()

const config = computed(() => ({
  host: { title: 'Host 상세', list: '/assets/server/hosts', name: asset.value.hostName, env: asset.value.environment },
  database: { title: 'Database 상세', list: '/assets/databases', name: asset.value.name, env: asset.value.env },
  k8s: { title: 'K8s Cluster 상세', list: '/containers/k8s/clusters', name: asset.value.name, env: asset.value.env }
}[props.resourceType]))

const status = computed(() => {
  if (props.resourceType === 'host') return asset.value.aliveStatus === 1 ? ['온라인', 'success'] : ['오프라인', 'danger']
  if (props.resourceType === 'database') return asset.value.connectStatus === 1 ? ['연결됨', 'success'] : asset.value.connectStatus === 2 ? ['연결 오류', 'danger'] : ['미검사', 'info']
  return asset.value.status === 'running' ? ['실행 중', 'success'] : asset.value.status === 'offline' ? ['오프라인', 'danger'] : ['주의', 'warning']
})

const basics = computed(() => {
  if (props.resourceType === 'host') return [
    ['Host 이름', asset.value.hostName], ['SSH 주소', `${asset.value.sshIp || '-'}:${asset.value.sshPort || 22}`],
    ['OS', asset.value.os], ['아키텍처', asset.value.arch], ['공인 IP', asset.value.publicIp], ['사설 IP', asset.value.privateIp]
  ]
  if (props.resourceType === 'database') return [
    ['유형', asset.value.dbType], ['연결 주소', `${asset.value.host || '-'}:${asset.value.port || 3306}`],
    ['기본 DB', asset.value.dbName], ['계정', asset.value.username], ['버전', asset.value.version], ['접근 모드', asset.value.accessMode === 'readonly' ? '읽기 전용' : '읽기/쓰기']
  ]
  return [
    ['API Server', asset.value.apiServer], ['Kubernetes 버전', asset.value.version], ['Node 수', asset.value.nodeCount],
    ['접속 방식', asset.value.connectionMode === 'gateway' ? 'Gateway 경유' : '직접 연결'], ['최근 동기화', formatDateTime(asset.value.lastSyncAt)], ['생성 시각', formatDateTime(asset.value.createTime)]
  ]
})

const gatewayName = computed(() => asset.value.gateway?.name || asset.value.gatewayName || '-')
const groupNames = computed(() => (asset.value.hostGroups || []).map((item) => item.name).join(', ') || asset.value.group?.name || '-')

function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false }).replaceAll('/', '-')
}

function actionText(action) {
  return ({ create: '생성', update: '수정', delete: '삭제', sync: '동기화', test: '연결 검사' })[action] || action
}

async function loadData() {
  loading.value = true
  try {
    const id = Number(route.params.id)
    const loaders = { host: assetHostInfo, database: assetDatabaseInfo, k8s: queryK8sClusterInfo }
    const [detail, logs] = await Promise.all([
      loaders[props.resourceType](id),
      queryAssetChangeLogs({ resourceType: props.resourceType, resourceId: id, limit: 30 })
    ])
    asset.value = detail || {}
    changes.value = logs || []
  } finally {
    loading.value = false
  }
}

function enterConsole() {
  if (props.resourceType === 'database') router.push({ name: 'DatabaseWorkbench', params: { id: route.params.id } })
  if (props.resourceType === 'k8s') router.push({ path: '/containers/k8s/overview', query: { clusterId: route.params.id } })
}

onMounted(loadData)
</script>

<template>
  <div v-loading="loading" class="asset-detail-page">
    <header class="detail-header">
      <div class="detail-title">
        <el-button :icon="ArrowLeft" circle title="뒤로" @click="router.push(config.list)" />
        <div>
          <div class="detail-kicker">{{ config.title }}</div>
          <h2>{{ config.name || '-' }}</h2>
        </div>
        <el-tag :type="status[1]" effect="light">{{ status[0] }}</el-tag>
      </div>
      <el-button v-if="resourceType !== 'host'" type="primary" :icon="Monitor" @click="enterConsole">
        {{ resourceType === 'database' ? 'DBMS Workbench로 이동' : 'Cluster 콘솔로 이동' }}
      </el-button>
    </header>

    <section class="detail-section">
      <div class="section-heading"><h3>기본 정보</h3><span>자산 식별 및 연결 정보</span></div>
      <el-descriptions :column="3" border>
        <el-descriptions-item v-for="item in basics" :key="item[0]" :label="item[0]">{{ item[1] ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="소속 Environment">{{ environmentName(config.env) }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType === 'host'" label="Host Group">{{ groupNames }}</el-descriptions-item>
        <el-descriptions-item label="접속 Gateway"><el-icon><Connection /></el-icon> {{ gatewayName }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType !== 'database'" label="최근 검사">{{ formatDateTime(asset.lastCheckTime || asset.lastSyncAt) }}</el-descriptions-item>
        <el-descriptions-item label="수정 시간">{{ formatDateTime(asset.updateTime) }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType === 'k8s'" label="Label" :span="3">
          <el-tag v-for="tag in asset.tags || []" :key="tag" class="asset-tag" effect="plain">{{ tag }}</el-tag>
          <span v-if="!asset.tags?.length">-</span>
        </el-descriptions-item>
        <el-descriptions-item label="비고" :span="3">{{ asset.description || '-' }}</el-descriptions-item>
      </el-descriptions>
    </section>

    <HostMetrics v-if="resourceType === 'host'" :host-id="Number(route.params.id)" />
    <DatabaseMetrics v-if="resourceType === 'database'" :database-id="Number(route.params.id)" :enabled="Boolean(asset.monitorEnabled)" />

    <section class="detail-section">
      <div class="section-heading"><h3>최근 변경</h3><span>핵심 자산 작업을 기록해 추적을 돕습니다</span></div>
      <el-table :data="changes" empty-text="변경 기록이 없습니다">
        <el-table-column label="시각" width="190"><template #default="{ row }">{{ formatDateTime(row.createTime) }}</template></el-table-column>
        <el-table-column label="동작" width="110"><template #default="{ row }"><el-tag effect="plain">{{ actionText(row.action) }}</el-tag></template></el-table-column>
        <el-table-column prop="summary" label="변경 내용" min-width="280" />
        <el-table-column label="작업자" width="160"><template #default="{ row }"><el-icon><EditPen /></el-icon> {{ row.operator || 'system' }}</template></el-table-column>
      </el-table>
    </section>
  </div>
</template>

<style scoped>
.asset-detail-page { display: flex; flex-direction: column; gap: 16px; }
.detail-header, .detail-section { background: #fff; border: 1px solid #e3eaf5; border-radius: 8px; }
.detail-header { min-height: 92px; padding: 18px 22px; display: flex; align-items: center; justify-content: space-between; }
.detail-title { display: flex; align-items: center; gap: 14px; }
.detail-title h2 { margin: 3px 0 0; color: #0f2445; font-size: 24px; letter-spacing: 0; }
.detail-kicker, .section-heading span { color: #7b8ca8; font-size: 13px; }
.detail-section { padding: 20px; }
.section-heading { display: flex; align-items: baseline; gap: 12px; margin-bottom: 16px; }
.section-heading h3 { margin: 0; color: #13294b; font-size: 17px; letter-spacing: 0; }
.asset-tag { margin-right: 6px; }
</style>
