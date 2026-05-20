<script setup>
import { computed, onMounted, ref } from 'vue'
import { profile, queryAdminList, queryRoleList, queryDeptList, queryPostList } from '../api/system'
import { t } from '../utils/i18n'

const currentUser = ref({})
const stats = ref([
  { title: t('userCount'), value: 0, note: t('userCountNote') },
  { title: t('roleCount'), value: 0, note: t('roleCountNote') },
  { title: t('deptCount'), value: 0, note: t('deptCountNote') },
  { title: t('postCount'), value: 0, note: t('postCountNote') }
])

const welcome = computed(() => currentUser.value.nickname || currentUser.value.username || '管理员')

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
      <div>
        <p class="hero-kicker">OPS-ADMIN</p>
        <h1>{{ t('welcomeBack', { name: welcome }) }}</h1>
        <p class="hero-text">{{ t('dashboardDesc') }}</p>
      </div>
      <div class="hero-badge">
        <span>System</span>
        <span>Config</span>
        <span>Audit</span>
      </div>
    </section>

    <section class="stat-grid">
      <article v-for="item in stats" :key="item.title" class="stat-card">
        <span>{{ item.title }}</span>
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
  padding: 28px 32px;
  border-radius: 24px;
  color: #fff;
  background:
    radial-gradient(circle at top right, rgba(101, 196, 255, 0.35), transparent 26%),
    linear-gradient(135deg, #222c61 0%, #405bc7 55%, #4f7dff 100%);
  box-shadow: 0 18px 40px rgba(46, 73, 166, 0.24);
}

.hero-kicker {
  margin: 0 0 10px;
  letter-spacing: 0.18em;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.72);
}

.hero-card h1 {
  margin: 0;
  font-size: 34px;
}

.hero-text {
  max-width: 620px;
  margin: 14px 0 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.75;
}

.hero-badge {
  display: grid;
  gap: 12px;
}

.hero-badge span {
  padding: 10px 18px;
  border-radius: 999px;
  text-align: center;
  background: rgba(255, 255, 255, 0.14);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.stat-card {
  padding: 22px;
  border-radius: 20px;
  background: #fff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.06);
  border: 1px solid #e8edf6;
}

.stat-card span {
  display: block;
  color: #64748b;
}

.stat-card strong {
  display: block;
  margin: 16px 0 10px;
  font-size: 34px;
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
