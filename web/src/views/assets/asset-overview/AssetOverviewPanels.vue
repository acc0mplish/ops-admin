<script setup>
// I5-J (i5-plan §3.8·CW-4) — AssetOverview.vue 849행 분할 자식(분석 패널 —
// 건강·그룹·분포·클러스터). 원본 좌표: 템플릿 :249-383(overview-main 섹션)·
// healthCards :75-103·분포 computed :105-107·ratio :109-113·
// clusterStatusType :145-150·clusterStatusText :152-157·openGroupHosts
// :166-174·openCluster :180-182·CSS .overview-column :593-596·.panel-status-chip
// :628-642·.health-* :644-686·.group-* :688-727·.distribution-* :729-784.
// `page` 주입(§5 #11 승계) — overview는 부모 로드 상태를 공유한다.
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Warning } from '@element-plus/icons-vue'
import { at } from '../../../utils/asset-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

const router = useRouter()

// 원본 :75-103
const healthCards = computed(() => {
  const health = props.page.overview.health || {}
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

// 원본 :105-107
const providerDistribution = computed(() => props.page.overview.distributions?.providers || [])
const environmentDistribution = computed(() => props.page.overview.distributions?.environments || [])
const hasEnvironmentDistribution = computed(() => environmentDistribution.value.length > 0)

// 원본 :109-113
function ratio(count, list) {
  const total = (list || []).reduce((sum, item) => sum + (item.count || 0), 0)
  if (!total) return 0
  return Math.max(10, Math.round((count / total) * 100))
}

// 원본 :145-150
function clusterStatusType(value) {
  if (value === 'running') return 'success'
  if (value === 'warning') return 'warning'
  if (value === 'error') return 'danger'
  return 'info'
}

// 원본 :152-157
function clusterStatusText(value) {
  if (value === 'running') return at('statusRunning')
  if (value === 'warning') return at('attentionStatus')
  if (value === 'error') return at('unusableStatus')
  return at('unknownStatus')
}

// 원본 :166-174
function openGroupHosts(group) {
  router.push({
    path: '/assets/server/hosts',
    query: {
      groupId: group.id,
      groupName: group.name
    }
  })
}

// 원본 :180-182
function openCluster(item) {
  router.push(`/containers/k8s/clusters/${item.id}/detail`)
}
</script>

<template>
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
        <div v-if="page.overview.topGroups?.length" class="group-list">
          <button
            v-for="item in page.overview.topGroups"
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

        <el-table :data="page.overview.recentClusters || []" size="small" class="compact-table">
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
</template>

<style scoped>
/* 원본 :586-591 — .detail-grid은 AssetOverviewDetail 자식과 공유(복사 1건) */
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

/* 원본 :598-606 — .detail-card은 AssetOverviewDetail 자식과 공유(복사 1건) */
.page-card,
.panel-card,
.detail-card {
  padding: 20px;
  border-radius: 10px;
  background: #fff;
  border: 1px solid #e7edf8;
  box-shadow: 0 2px 5px rgba(20, 34, 58, 0.035);
}

/* 원본 :608-626 — AssetOverviewDetail 자식과 공유(복사 3건) */
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

/* 원본 :786-789 — AssetOverviewDetail 자식과 공유(복사 1건) */
.compact-table :deep(.el-table__cell) {
  padding-top: 10px;
  padding-bottom: 10px;
}

/* 원본 :796-799 — .entity-cell small은 AssetOverviewDetail 자식 소속(복사 1건) */
.entity-cell small,
.sub-line {
  color: #94a3b8;
}

/* 원본 :801-812 — AssetOverviewDetail 자식과 공유(복사 2건) */
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

/* 원본 :825-839 미디어 1100 중 health-grid 소속 셀렉터만 자식 귀속 —
    * .hero-card/.hero-side는 부모 잔류 */
@media (max-width: 1100px) {
  .health-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

/* 원본 :841-848 미디어 820 분할 — .distribution-grid/.health-grid 소속만 자식,
    * .summary-grid/.hero-side는 부모 잔류 */
@media (max-width: 820px) {
  .distribution-grid,
  .health-grid {
    grid-template-columns: 1fr;
  }
}

/* 원본 :819-822 미디어 1500 — .overview-main/.detail-grid 규칙(AssetOverviewDetail
    * 자식과 공유 복사 1건) */
@media (max-width: 1500px) {
  .overview-main,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
