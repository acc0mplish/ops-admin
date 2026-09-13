<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { at } from '../../utils/asset-i18n'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Connection, FolderOpened, Key, Monitor, Coin, Grid } from '@element-plus/icons-vue'
import { queryAssetOverview } from '../../api/asset'
// I5-J (i5-plan §3.8·CW-4) — 849행 분할 잔류본. 분석 패널(건강·그룹·분포·클러스터)은
// asset-overview/AssetOverviewPanels.vue, 최근 항목(호스트·데이터베이스 테이블)은
// AssetOverviewDetail.vue로 이동(원본 좌표는 커밋 메시지·자식 파일 헤더). 자식은
// :page 주입(§5 #11 승계).
import AssetOverviewPanels from './asset-overview/AssetOverviewPanels.vue'
import AssetOverviewDetail from './asset-overview/AssetOverviewDetail.vue'

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

// I5-J 자식 주입 번들 — reactive 래핑으로 ref가 언랩되어 자식 템플릿의
// page.x 재배선이 동작한다(§5 #11).
const page = reactive({
  overview
})

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

    <AssetOverviewPanels :page="page" />

    <AssetOverviewDetail :page="page" />
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

/* 원본 :814-818 미디어 1500 — .summary-grid 소속만 부모 잔류.
    * .overview-main/.detail-grid 규칙은 자식 2종으로 이동 */
@media (max-width: 1500px) {
  .summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

/* 원본 :826-834 미디어 1100 — hero 소속 셀렉터만 부모 잔류.
    * .health-grid은 AssetOverviewPanels 자식 */
@media (max-width: 1100px) {
  .hero-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-side {
    width: 100%;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

/* 원본 :841-848 미디어 820 분할 — .summary-grid/.hero-side 소속만 부모 잔류.
    * .distribution-grid/.health-grid은 AssetOverviewPanels 자식 */
@media (max-width: 820px) {
  .summary-grid,
  .hero-side {
    grid-template-columns: 1fr;
  }
}
</style>
