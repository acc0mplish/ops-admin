<script setup>
import { computed, onMounted, ref } from 'vue'
import { profile, queryAdminList, queryRoleList, queryDeptList, queryPostList } from '../api/system'
import { t } from '../utils/i18n-runtime'

const currentUser = ref({})
const stats = ref([
  { title: t('userCount'), value: 0, note: t('userCountNote') },
  { title: t('roleCount'), value: 0, note: t('roleCountNote') },
  { title: t('deptCount'), value: 0, note: t('deptCountNote') },
  { title: t('postCount'), value: 0, note: t('postCountNote') }
])

const welcome = computed(() => currentUser.value.nickname || currentUser.value.username || t('administrator'))

onMounted(async () => {
  const [userInfo, adminRes, roleRes, deptRes, postRes] = await Promise.all([
    profile(),
    queryAdminList({ pageNum: 1, pageSize: 1 }),
    queryRoleList(),
    queryDeptList(),
    queryPostList()
  ])

  currentUser.value = userInfo.user || {}
  stats.value = [
    { title: t('userCount'), value: adminRes.total || 0, note: t('userCountNote') },
    { title: t('roleCount'), value: roleRes.total || 0, note: t('roleCountNote') },
    { title: t('deptCount'), value: (deptRes || []).length, note: t('deptCountNote') },
    { title: t('postCount'), value: postRes.total || 0, note: t('postCountNote') }
  ]
})
</script>

<template>
  <div class="dashboard-page">
    <section class="hero-card">
      <div class="hero-copy">
        <p class="hero-kicker">{{ t('systemGovernance') }}</p>
        <h1>{{ t('welcomeBack', { name: welcome }) }}</h1>
        <p class="hero-text">{{ t('dashboardDesc') }}</p>
      </div>
      <div class="hero-badge" :aria-label="t('consoleScope')">
        <p>{{ t('consoleScope') }}</p>
        <span><i></i>{{ t('systemGovernance') }}</span>
        <span><i></i>{{ t('permissionConfiguration') }}</span>
        <span><i></i>{{ t('auditTrail') }}</span>
      </div>
    </section>

    <section class="stat-grid">
      <article v-for="(item, index) in stats" :key="item.title" class="stat-card" :class="`metric-${index + 1}`">
        <span class="stat-label">{{ item.title }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.note }}</small>
      </article>
    </section>
  </div>
</template>

<style scoped>
.dashboard-page {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 26px 30px;
  border: 1px solid rgba(142, 178, 218, 0.2);
  border-radius: 14px;
  color: #fff;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    radial-gradient(circle at 88% 8%, rgba(24, 168, 160, 0.22), transparent 24%),
    #132640;
  background-size: 24px 24px, 24px 24px, auto, auto;
  box-shadow: none;
}

.hero-kicker {
  margin: 0 0 10px;
  letter-spacing: 0.18em;
  font-size: 12px;
  color: #9db1c9;
}

.hero-card h1 {
  margin: 0;
  font-size: 30px;
  letter-spacing: -0.025em;
}

.hero-text {
  max-width: 620px;
  margin: 14px 0 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.75;
}

.hero-badge {
  display: grid;
  min-width: 174px;
  gap: 8px;
  padding: 13px 14px;
  border: 1px solid rgba(192, 214, 239, 0.16);
  border-radius: 10px;
  background: rgba(6, 19, 37, 0.32);
}

.hero-badge p { margin: 0 0 2px; color: #8ea3bd; font-size: 10px; font-weight: 700; letter-spacing: .12em; }
.hero-badge span {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #dbe8f8;
  font-size: 13px;
}
.hero-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #2dd4bf;
  box-shadow: 0 0 0 3px rgba(45, 212, 191, .12);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.stat-card {
  position: relative;
  overflow: hidden;
  padding: 22px;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 2px 7px rgba(15, 23, 42, 0.045);
  border: 1px solid #e8edf6;
}

.stat-card::before {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 3px;
  background: #356ae6;
  content: '';
}
.stat-card.metric-2::before { background: #0f766e; }
.stat-card.metric-3::before { background: #7c5ce3; }
.stat-card.metric-4::before { background: #b7791f; }
.stat-label {
  display: block;
  color: #64748b;
}

.stat-card strong {
  display: block;
  margin: 16px 0 10px;
  font-size: 32px;
  font-variant-numeric: tabular-nums;
  color: #111827;
}

.stat-card small {
  color: #94a3b8;
}

@media (max-width: 1100px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .hero-card {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 720px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
