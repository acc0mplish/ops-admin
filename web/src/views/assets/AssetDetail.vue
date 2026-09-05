<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Connection, EditPen, Monitor } from '@element-plus/icons-vue'
import { assetDatabaseInfo, assetHostInfo, queryAssetChangeLogs } from '../../api/asset'
import { queryK8sClusterInfo } from '../../api/k8s'
import { useEnvironmentOptions } from '../../composables/useEnvironmentOptions'
import { at } from '../../utils/asset-i18n'
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
  host: { title: at('hostDetailTitle'), list: '/assets/server/hosts', name: asset.value.hostName, env: asset.value.environment },
  database: { title: at('databaseDetailTitle'), list: '/assets/databases', name: asset.value.name, env: asset.value.env },
  k8s: { title: at('k8sDetailTitle'), list: '/containers/k8s/clusters', name: asset.value.name, env: asset.value.env }
}[props.resourceType]))

const status = computed(() => {
  if (props.resourceType === 'host') return asset.value.aliveStatus === 1 ? [at('online'), 'success'] : [at('offline'), 'danger']
  if (props.resourceType === 'database') return asset.value.connectStatus === 1 ? [at('dbConnected'), 'success'] : asset.value.connectStatus === 2 ? [at('dbConnectError'), 'danger'] : [at('notInspected'), 'info']
  return asset.value.status === 'running' ? [at('statusRunning'), 'success'] : asset.value.status === 'offline' ? [at('offline'), 'danger'] : [at('attentionStatus'), 'warning']
})

const basics = computed(() => {
  if (props.resourceType === 'host') return [
    [at('hostNameLabel'), asset.value.hostName], [at('sshAddressLabel'), `${asset.value.sshIp || '-'}:${asset.value.sshPort || 22}`],
    ['OS', asset.value.os], [at('archLabel'), asset.value.arch], [at('publicIpColumn'), asset.value.publicIp], [at('privateIpColumn'), asset.value.privateIp]
  ]
  if (props.resourceType === 'database') return [
    [at('typeColumn'), asset.value.dbType], [at('connAddrColumn'), `${asset.value.host || '-'}:${asset.value.port || 3306}`],
    [at('defaultDbColumn'), asset.value.dbName], [at('accountColumn'), asset.value.username], [at('versionColumn'), asset.value.version], [at('accessModeField'), asset.value.accessMode === 'readonly' ? at('readOnly') : at('readWrite')]
  ]
  return [
    ['API Server', asset.value.apiServer], [at('k8sVersionLabel'), asset.value.version], [at('nodeCountColumn'), asset.value.nodeCount],
    [at('accessModeLabel'), asset.value.connectionMode === 'gateway' ? at('viaGateway') : at('directConnection')], [at('lastSyncLabel'), formatDateTime(asset.value.lastSyncAt)], [at('createdAtColumn'), formatDateTime(asset.value.createTime)]
  ]
})

const gatewayName = computed(() => asset.value.gateway?.name || asset.value.gatewayName || '-')
const groupNames = computed(() => (asset.value.hostGroups || []).map((item) => item.name).join(', ') || asset.value.group?.name || '-')

function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false }).replaceAll('/', '-')
}

function actionText(action) {
  return ({ create: at('createAction'), update: at('editAction2'), delete: at('delete'), sync: at('sync'), test: at('connectionCheck') })[action] || action
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
        <el-button :icon="ArrowLeft" circle :title="at('back')" @click="router.push(config.list)" />
        <div>
          <div class="detail-kicker">{{ config.title }}</div>
          <h2>{{ config.name || '-' }}</h2>
        </div>
        <el-tag :type="status[1]" effect="light">{{ status[0] }}</el-tag>
      </div>
      <el-button v-if="resourceType !== 'host'" type="primary" :icon="Monitor" @click="enterConsole">
        {{ resourceType === 'database' ? at('goWorkbench') : at('goClusterConsole') }}
      </el-button>
    </header>

    <section class="detail-section">
      <div class="section-heading"><h3>{{ at('basicInfoTitle') }}</h3><span>{{ at('basicInfoDesc') }}</span></div>
      <el-descriptions :column="3" border>
        <el-descriptions-item v-for="item in basics" :key="item[0]" :label="item[0]">{{ item[1] ?? '-' }}</el-descriptions-item>
        <el-descriptions-item :label="at('environmentLabel')">{{ environmentName(config.env) }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType === 'host'" label="Host Group">{{ groupNames }}</el-descriptions-item>
        <el-descriptions-item :label="at('accessGatewayLabel')"><el-icon><Connection /></el-icon> {{ gatewayName }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType !== 'database'" :label="at('lastCheckColumn')">{{ formatDateTime(asset.lastCheckTime || asset.lastSyncAt) }}</el-descriptions-item>
        <el-descriptions-item :label="at('updatedAtColumn')">{{ formatDateTime(asset.updateTime) }}</el-descriptions-item>
        <el-descriptions-item v-if="resourceType === 'k8s'" label="Label" :span="3">
          <el-tag v-for="tag in asset.tags || []" :key="tag" class="asset-tag" effect="plain">{{ tag }}</el-tag>
          <span v-if="!asset.tags?.length">-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="at('noteLabel')" :span="3">{{ asset.description || '-' }}</el-descriptions-item>
      </el-descriptions>
    </section>

    <HostMetrics v-if="resourceType === 'host'" :host-id="Number(route.params.id)" />
    <DatabaseMetrics v-if="resourceType === 'database'" :database-id="Number(route.params.id)" :enabled="Boolean(asset.monitorEnabled)" />

    <section class="detail-section">
      <div class="section-heading"><h3>{{ at('recentChangesTitle') }}</h3><span>{{ at('recentChangesDesc') }}</span></div>
      <el-table :data="changes" :empty-text="at('noChangeLogs')">
        <el-table-column :label="at('timeColumn')" width="190"><template #default="{ row }">{{ formatDateTime(row.createTime) }}</template></el-table-column>
        <el-table-column :label="at('actionColumn')" width="110"><template #default="{ row }"><el-tag effect="plain">{{ actionText(row.action) }}</el-tag></template></el-table-column>
        <el-table-column prop="summary" :label="at('changeSummaryColumn')" min-width="280" />
        <el-table-column :label="at('operatorColumn')" width="160"><template #default="{ row }"><el-icon><EditPen /></el-icon> {{ row.operator || 'system' }}</template></el-table-column>
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
