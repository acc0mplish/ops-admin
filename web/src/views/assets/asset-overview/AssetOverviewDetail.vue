<script setup>
// I5-J (i5-plan §3.8·CW-4) — AssetOverview.vue 849행 분할 자식(최근 항목 —
// 호스트·데이터베이스 테이블). 원본 좌표: 템플릿 :385-461(detail-grid 섹션)·
// hostStatusType :115-119·hostStatusText :121-125·authStatusText :127-131·
// databaseStatusType :133-137·databaseStatusText :139-143·formatTime :159-164·
// openDatabase :176-178·openHost :184-186·CSS .compact-table :786-789·
// .entity-cell :791-799·.link-button :801-812.
// `page` 주입(§5 #11 승계) — overview는 부모 로드 상태를 공유한다.
import { useRouter } from 'vue-router'
import { at } from '../../../utils/asset-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

const router = useRouter()

// 원본 :115-119
function hostStatusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

// 원본 :121-125
function hostStatusText(value) {
  if (value === 1) return at('online')
  if (value === 2) return at('offline')
  return at('unknownStatus')
}

// 원본 :127-131
function authStatusText(value) {
  if (value === 1) return at('authSuccess')
  if (value === 2) return at('authFailed')
  return at('verificationPending')
}

// 원본 :133-137
function databaseStatusType(value) {
  if (value === 1) return 'success'
  if (value === 2) return 'danger'
  return 'info'
}

// 원본 :139-143
function databaseStatusText(value) {
  if (value === 1) return at('dbHealthy')
  if (value === 2) return at('dbUnhealthy')
  return at('notInspected')
}

// 원본 :159-164
function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

// 원본 :176-178
function openDatabase(item) {
  router.push(`/assets/databases/${item.id}/detail`)
}

// 원본 :184-186
function openHost(item) {
  router.push(`/assets/server/hosts/${item.id}/detail`)
}
</script>

<template>
  <section class="detail-grid">
    <article class="page-card detail-card">
      <div class="panel-header">
        <div>
          <h3>{{ at('recentHostsTitle') }}</h3>
          <p>{{ at('recentHostsDesc') }}</p>
        </div>
        <el-button link type="primary" @click="router.push('/assets/server/hosts')">{{ at('hostManageLink') }}</el-button>
      </div>

      <el-table :data="page.overview.recentHosts || []" size="small" class="compact-table">
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

      <el-table :data="page.overview.recentDatabases || []" size="small" class="compact-table">
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
</template>

<style scoped>
/* 원본 :586-591 — .overview-main은 AssetOverviewPanels 자식과 공유(복사 1건) */
.overview-main,
.detail-grid {
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  gap: 18px;
}

/* 원본 :598-606 — .panel-card은 AssetOverviewPanels 자식과 공유(복사 1건) */
.page-card,
.panel-card,
.detail-card {
  padding: 20px;
  border-radius: 10px;
  background: #fff;
  border: 1px solid #e7edf8;
  box-shadow: 0 2px 5px rgba(20, 34, 58, 0.035);
}

/* 원본 :608-626 — AssetOverviewPanels 자식과 공유(복사 3건) */
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

/* 원본 :786-789 — AssetOverviewPanels 자식과 공유(복사 1건) */
.compact-table :deep(.el-table__cell) {
  padding-top: 10px;
  padding-bottom: 10px;
}

.entity-cell strong,
.entity-cell small {
  display: block;
}

/* 원본 :796-799 — .sub-line은 AssetOverviewPanels 자식과 공유(복사 1건) */
.entity-cell small,
.sub-line {
  color: #94a3b8;
}

/* 원본 :801-812 — AssetOverviewPanels 자식과 공유(복사 2건) */
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

/* 원본 :819-822 미디어 1500 — .overview-main/.detail-grid 규칙
   (AssetOverviewPanels 자식과 공유 복사 1건) */
@media (max-width: 1500px) {
  .overview-main,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
