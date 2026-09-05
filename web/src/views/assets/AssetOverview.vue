<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { at } from '../../utils/asset-i18n'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Connection, FolderOpened, Key, Monitor, Coin, Warning, Grid } from '@element-plus/icons-vue'
import { queryAssetOverview } from '../../api/asset'

const router = useRouter()
const loading = ref(false)
const overview = ref({
  summary: {},
  health: {},
  distributions: { providers: [], environments: [] },
  topGroups: [],
  recentHosts: [],
  recentDatabases: [],
  recentClusters: []
})

const summaryCards = computed(() => {
  const summary = overview.value.summary || {}
  return [
    {
      key: 'hosts',
      title: at('hostsCardTitle'),
      value: summary.hostTotal || 0,
      note: at('hostsCardNote', { online: summary.hostOnline || 0, offline: summary.hostOffline || 0 }),
      icon: Monitor,
      action: () => router.push('/assets/server/hosts')
    },
    {
      key: 'groups',
      title: 'Host Group',
      value: summary.groupTotal || 0,
      note: at('groupsCardNote'),
      icon: FolderOpened,
      action: () => router.push('/assets/server/groups')
    },
    {
      key: 'credentials',
      title: 'Credential',
      value: summary.credentialTotal || 0,
      note: at('enabledCount', { count: summary.credentialEnabled || 0 }),
      icon: Key,
      action: () => router.push('/assets/server/credentials')
    },
    {
      key: 'cloudAccounts',
      title: 'Cloud Account',
      value: summary.cloudAccountTotal || 0,
      note: at('availableCount', { count: summary.cloudAccountEnabled || 0 }),
      icon: Connection,
      action: () => router.push('/assets/server/cloud-accounts')
    },
    {
      key: 'databases',
      title: 'Database',
      value: summary.databaseTotal || 0,
      note: at('healthyCount', { count: summary.databaseHealthy || 0 }),
      icon: Coin,
      action: () => router.push('/assets/databases')
    },
    {
      key: 'k8s',
      title: 'K8s Cluster',
      value: summary.k8sClusterTotal || 0,
      note: at('k8sCardNote', { online: summary.k8sClusterOnline || 0, nodes: summary.k8sNodeTotal || 0 }),
      icon: Grid,
      action: () => router.push('/containers/k8s/clusters')
    }
  ]
})

const healthCards = computed(() => {
  const health = overview.value.health || {}
  return [
    {
      title: at('healthOfflineTitle'),
      value: health.offlineHosts || 0,
      tone: health.offlineHosts ? 'danger' : 'normal',
      desc: at('healthOfflineDesc')
    },
    {
      title: at('healthAuthTitle'),
      value: health.authFailedHosts || 0,
      tone: health.authFailedHosts ? 'warning' : 'normal',
      desc: at('healthAuthDesc')
    },
    {
      title: at('healthDbTitle'),
      value: health.abnormalDatabases || 0,
      tone: health.abnormalDatabases ? 'danger' : 'normal',
      desc: at('healthDbDesc')
    },
    {
      title: at('healthClusterTitle'),
      value: health.abnormalClusters || 0,
      tone: health.abnormalClusters ? 'warning' : 'normal',
      desc: at('healthClusterDesc')
    }
  ]
})

const providerDistribution = computed(() => overview.value.distributions?.providers || [])
const environmentDistribution = computed(() => overview.value.distributions?.environments || [])
const hasEnvironmentDistribution = computed(() => environmentDistribution.value.length > 0)

function ratio(count, list) {
  const total = (list || []).reduce((sum, item) => sum + (item.count || 0), 0)
  if (!total) return 0
  return Math.max(10, Math.round((count / total) * 100))
}

function hostStatusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

function hostStatusText(value) {
  if (value === 1) return at('online')
  if (value === 2) return at('offline')
  return at('unknownStatus')
}

function authStatusText(value) {
  if (value === 1) return at('authSuccess')
  if (value === 2) return at('authFailed')
  return at('verificationPending')
}

function databaseStatusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

function databaseStatusText(value) {
  if (value === 1) return at('dbHealthy')
  if (value === 2) return at('dbUnhealthy')
  return at('notInspected')
}

function clusterStatusType(value) {
  if (value === 'running') return 'success'
  if (value === 'warning') return 'warning'
  if (value === 'error') return 'danger'
  return 'info'
}

function clusterStatusText(value) {
  if (value === 'running') return at('statusRunning')
  if (value === 'warning') return at('attentionStatus')
  if (value === 'error') return at('unusableStatus')
  return at('unknownStatus')
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function openGroupHosts(group) {
  router.push({
    path: '/assets/server/hosts',
    query: {
      groupId: group.id,
      groupName: group.name
    }
  })
}

function openDatabase(item) {
  router.push(`/assets/databases/${item.id}/detail`)
}

function openCluster(item) {
  router.push(`/containers/k8s/clusters/${item.id}/detail`)
}

function openHost(item) {
  router.push(`/assets/server/hosts/${item.id}/detail`)
}

async function loadOverview() {
  loading.value = true
  try {
    overview.value = await queryAssetOverview()
  } finally {
    loading.value = false
  }
}

onMounted(loadOverview)
</script>

<template>
  <div v-loading="loading" class="asset-overview-page">
    <section class="hero-card">
      <div class="hero-copy">
        <p class="hero-kicker">{{ uiT('assetControl') }}</p>
        <h1>Asset Overview</h1>
        <p class="hero-text">
          {{ at('heroText') }}
        </p>
      </div>
      <div class="hero-side">
        <article class="hero-side-card">
          <span>{{ at('heroOnlineHosts') }}</span>
          <strong>{{ overview.summary?.hostOnline || 0 }}</strong>
        </article>
        <article class="hero-side-card">
          <span>{{ at('heroDbAvailable') }}</span>
          <strong>{{ overview.summary?.databaseHealthy || 0 }}</strong>
        </article>
        <article class="hero-side-card">
          <span>K8s Node</span>
          <strong>{{ overview.summary?.k8sNodeTotal || 0 }}</strong>
        </article>
        <article class="hero-side-card" :class="{ attention: overview.health?.incompleteAssets }">
          <span>{{ at('heroInfoPending') }}</span>
          <strong>{{ overview.health?.incompleteAssets || 0 }}</strong>
        </article>
      </div>
    </section>

    <section class="summary-grid">
      <article
        v-for="item in summaryCards"
        :key="item.key"
        class="summary-card"
        @click="item.action"
      >
        <div class="summary-head">
          <div class="summary-icon">
            <component :is="item.icon" />
          </div>
          <el-button link type="primary">{{ at('go') }}</el-button>
        </div>
        <span>{{ item.title }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.note }}</small>
      </article>
    </section>

    <section class="overview-main">
      <div class="overview-column">
        <article class="page-card panel-card">
          <div class="panel-header">
            <div>
              <h3>{{ at('healthAlertTitle') }}</h3>
              <p>{{ at('healthAlertDesc') }}</p>
            </div>
            <div class="panel-status-chip">
              <Warning />
              <span>{{ at('assetHealthChip') }}</span>
            </div>
          </div>
          <div class="health-grid">
            <div v-for="item in healthCards" :key="item.title" class="health-item" :class="item.tone">
              <span>{{ item.title }}</span>
              <strong>{{ item.value }}</strong>
              <small>{{ item.desc }}</small>
            </div>
          </div>
        </article>

        <article class="page-card panel-card">
          <div class="panel-header">
            <div>
              <h3>{{ at('groupDistTitle') }}</h3>
              <p>{{ at('groupDistDesc') }}</p>
            </div>
            <el-button link type="primary" @click="router.push('/assets/server/groups')">{{ at('viewAll') }}</el-button>
          </div>
          <div v-if="overview.topGroups?.length" class="group-list">
            <button
              v-for="item in overview.topGroups"
              :key="item.id"
              class="group-row"
              @click="openGroupHosts(item)"
            >
              <div>
                <strong>{{ item.name }}</strong>
                <small>{{ item.code || at('noCode') }}</small>
              </div>
              <div class="group-meta">
                <span>{{ at('hostCountSuffix', { count: item.hostCount }) }}</span>
                <el-tag :type="item.status === 1 ? 'success' : 'info'" effect="light">
                  {{ item.status === 1 ? at('groupNormal') : at('groupDisabled') }}
                </el-tag>
              </div>
            </button>
          </div>
          <el-empty v-else :description="at('noGroupData')" />
        </article>
      </div>

      <div class="overview-column">
        <article class="page-card panel-card">
          <div class="panel-header">
            <div>
              <h3>{{ at('distTitle') }}</h3>
              <p>{{ at('distDesc') }}</p>
            </div>
          </div>
          <div class="distribution-grid">
            <section class="distribution-card">
              <header>
                <strong>{{ at('hostSourceTitle') }}</strong>
              </header>
              <div v-if="providerDistribution.length" class="distribution-list">
                <div v-for="item in providerDistribution" :key="item.name" class="distribution-row">
                  <div class="distribution-label">
                    <span>{{ item.name }}</span>
                    <strong>{{ item.count }}</strong>
                  </div>
                  <div class="distribution-track">
                    <div class="distribution-fill" :style="{ width: `${ratio(item.count, providerDistribution)}%` }" />
                  </div>
                </div>
              </div>
              <el-empty v-else :description="at('noProviderData')" :image-size="72" />
            </section>

            <section class="distribution-card">
              <header>
                <strong>{{ at('envDistTitle') }}</strong>
              </header>
              <div v-if="hasEnvironmentDistribution" class="distribution-list">
                <div v-for="item in environmentDistribution" :key="item.name" class="distribution-row">
                  <div class="distribution-label">
                    <span>{{ item.name }}</span>
                    <strong>{{ item.count }}</strong>
                  </div>
                  <div class="distribution-track">
                    <div class="distribution-fill secondary" :style="{ width: `${ratio(item.count, environmentDistribution)}%` }" />
                  </div>
                </div>
              </div>
              <div v-else class="distribution-placeholder">
                {{ at('noEnvDistText') }}
              </div>
            </section>
          </div>
        </article>

        <article class="page-card panel-card">
          <div class="panel-header">
            <div>
              <h3>{{ at('clusterStatusTitle') }}</h3>
              <p>{{ at('clusterStatusDesc') }}</p>
            </div>
            <el-button link type="primary" @click="router.push('/containers/k8s/clusters')">{{ at('clusterManageLink') }}</el-button>
          </div>

          <el-table :data="overview.recentClusters || []" size="small" class="compact-table">
            <el-table-column label="Cluster" min-width="180">
              <template #default="{ row }">
                <button class="link-button" @click="openCluster(row)">{{ row.name }}</button>
                <small class="sub-line">{{ row.apiServer || '-' }}</small>
              </template>
            </el-table-column>
            <el-table-column :label="at('versionColumn')" width="110">
              <template #default="{ row }">{{ row.version || '-' }}</template>
            </el-table-column>
            <el-table-column :label="at('nodeCountColumn')" width="90">
              <template #default="{ row }">{{ row.nodeCount || 0 }}</template>
            </el-table-column>
            <el-table-column :label="at('status')" width="110">
              <template #default="{ row }">
                <el-tag :type="clusterStatusType(row.status)" effect="light">
                  {{ clusterStatusText(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </article>
      </div>
    </section>

    <section class="detail-grid">
      <article class="page-card detail-card">
        <div class="panel-header">
          <div>
            <h3>{{ at('recentHostsTitle') }}</h3>
            <p>{{ at('recentHostsDesc') }}</p>
          </div>
          <el-button link type="primary" @click="router.push('/assets/server/hosts')">{{ at('hostManageLink') }}</el-button>
        </div>

        <el-table :data="overview.recentHosts || []" size="small" class="compact-table">
          <el-table-column label="Host" min-width="180">
            <template #default="{ row }">
              <div class="entity-cell">
                <button class="link-button" @click="openHost(row)">{{ row.hostName }}</button>
                <small>{{ row.sshIp || row.privateIp || row.publicIp || '-' }}</small>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="Host Group" min-width="160">
            <template #default="{ row }">
              {{ row.groupNames?.length ? row.groupNames.join(' / ') : '-' }}
            </template>
          </el-table-column>
          <el-table-column :label="at('status')" width="110">
            <template #default="{ row }">
              <el-tag :type="hostStatusType(row.aliveStatus)" effect="light">
                {{ hostStatusText(row.aliveStatus) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="at('authColumn')" width="110">
            <template #default="{ row }">
              {{ authStatusText(row.authStatus) }}
            </template>
          </el-table-column>
          <el-table-column :label="at('updatedAtColumn')" min-width="140">
            <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
          </el-table-column>
        </el-table>
      </article>

      <article class="page-card detail-card">
        <div class="panel-header">
          <div>
            <h3>{{ at('recentDbsTitle') }}</h3>
            <p>{{ at('recentDbsDesc') }}</p>
          </div>
          <el-button link type="primary" @click="router.push('/assets/databases')">{{ at('dbManageLink') }}</el-button>
        </div>

        <el-table :data="overview.recentDatabases || []" size="small" class="compact-table">
          <el-table-column label="Database" min-width="180">
            <template #default="{ row }">
              <button class="link-button" @click="openDatabase(row)">{{ row.name }}</button>
              <small class="sub-line">{{ row.dbName || '-' }}</small>
            </template>
          </el-table-column>
          <el-table-column :label="at('addressColumn')" min-width="180">
            <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
          </el-table-column>
          <el-table-column :label="at('typeColumn')" width="90">
            <template #default="{ row }">{{ (row.dbType || '').toUpperCase() || '-' }}</template>
          </el-table-column>
          <el-table-column :label="at('connStatusColumn')" width="120">
            <template #default="{ row }">
              <el-tag :type="databaseStatusType(row.connectStatus)" effect="light">
                {{ databaseStatusText(row.connectStatus) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="at('updatedAtColumn')" min-width="140">
            <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
          </el-table-column>
        </el-table>
      </article>
    </section>
  </div>
</template>

<style scoped>
.asset-overview-page {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 24px;
  border-radius: 12px;
  color: #fff;
  background: linear-gradient(118deg, #1b2d49 0%, #294f91 100%);
  box-shadow: 0 8px 20px rgba(28, 54, 97, 0.14);
}

.hero-copy h1 {
  margin: 0;
  font-size: 26px;
}

.hero-kicker {
  margin: 0 0 10px;
  font-size: 12px;
  letter-spacing: 0.18em;
  color: rgba(255, 255, 255, 0.72);
}

.hero-text {
  max-width: 700px;
  margin: 14px 0 0;
  line-height: 1.55;
  color: rgba(255, 255, 255, 0.84);
}

.hero-side {
  display: grid;
  gap: 12px;
  min-width: 520px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.hero-side-card {
  padding: 12px 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.1);
}

.hero-side-card span {
  display: block;
  color: rgba(255, 255, 255, 0.76);
}

.hero-side-card strong {
  display: block;
  margin-top: 8px;
  font-size: 22px;
}

.hero-side-card.attention { background: rgba(217, 140, 22, 0.28); }

.summary-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 16px;
}

.summary-card {
  padding: 16px;
  border: 1px solid #e6ecf7;
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 2px 5px rgba(20, 34, 58, 0.035);
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.summary-card:hover {
  transform: translateY(-1px);
  border-color: #bcd0f7;
  box-shadow: 0 8px 18px rgba(20, 34, 58, 0.08);
}

.summary-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.summary-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: #eef3ff;
  color: #4060da;
  font-size: 18px;
}

.summary-card span {
  display: block;
  color: #64748b;
}

.summary-card strong {
  display: block;
  margin: 12px 0 8px;
  font-size: 30px;
  color: #0f172a;
}

.summary-card small {
  color: #94a3b8;
  line-height: 1.6;
}

.overview-main,
.detail-grid {
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  gap: 18px;
}

.overview-column {
  display: grid;
  gap: 18px;
}

.page-card,
.panel-card,
.detail-card {
  padding: 20px;
  border-radius: 10px;
  background: #fff;
  border: 1px solid #e7edf8;
  box-shadow: 0 2px 5px rgba(20, 34, 58, 0.035);
}

.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.panel-header h3 {
  margin: 0;
  font-size: 20px;
  color: #0f172a;
}

.panel-header p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.7;
}

.panel-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 999px;
  background: #fff5f5;
  color: #dc2626;
  flex: 0 0 auto;
}

.panel-status-chip :deep(svg) {
  width: 14px;
  height: 14px;
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.health-item {
  padding: 14px 16px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid #e7edf8;
}

.health-item.warning {
  background: #fffaf0;
  border-color: #fde7ba;
}

.health-item.danger {
  background: #fff5f5;
  border-color: #fecaca;
}

.health-item span,
.health-item small {
  display: block;
}

.health-item span {
  color: #64748b;
}

.health-item strong {
  display: block;
  margin: 12px 0 8px;
  font-size: 28px;
  color: #0f172a;
}

.health-item small {
  color: #94a3b8;
  line-height: 1.6;
}

.group-list {
  display: grid;
  gap: 12px;
}

.group-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
  padding: 14px 16px;
  border: 1px solid #e7edf8;
  border-radius: 8px;
  background: #fafbfd;
  text-align: left;
  cursor: pointer;
}

.group-row:hover {
  border-color: #ccd8fb;
  background: #f7f9ff;
}

.group-row strong,
.group-row small {
  display: block;
}

.group-row small {
  margin-top: 6px;
  color: #94a3b8;
}

.group-meta {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #475569;
}

.distribution-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.distribution-card {
  padding: 18px;
  border-radius: 8px;
  background: #fafbfd;
  border: 1px solid #ecf1fb;
}

.distribution-card header {
  margin-bottom: 14px;
}

.distribution-list {
  display: grid;
  gap: 12px;
}

.distribution-row {
  display: grid;
  gap: 8px;
}

.distribution-label {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: #475569;
}

.distribution-track {
  height: 8px;
  border-radius: 999px;
  background: #e8eefb;
  overflow: hidden;
}

.distribution-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #4f7dff 0%, #6ea8ff 100%);
}

.distribution-fill.secondary {
  background: linear-gradient(90deg, #6d57d9 0%, #8d7dff 100%);
}

.distribution-placeholder {
  padding: 18px 0;
  color: #94a3b8;
  line-height: 1.8;
}

.compact-table :deep(.el-table__cell) {
  padding-top: 10px;
  padding-bottom: 10px;
}

.entity-cell strong,
.entity-cell small {
  display: block;
}

.entity-cell small,
.sub-line {
  color: #94a3b8;
}

.link-button {
  padding: 0;
  border: 0;
  background: transparent;
  color: #3661df;
  font-weight: 600;
  cursor: pointer;
}

.link-button:hover {
  color: #274fc8;
}

@media (max-width: 1500px) {
  .summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .overview-main,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1100px) {
  .hero-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-side {
    width: 100%;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .health-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 820px) {
  .summary-grid,
  .distribution-grid,
  .health-grid,
  .hero-side {
    grid-template-columns: 1fr;
  }
}
</style>
