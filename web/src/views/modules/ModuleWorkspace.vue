<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getAppByRoute } from '../../utils/apps'
import { t, translateRoute } from '../../utils/i18n'

const route = useRoute()

const app = computed(() => getAppByRoute(route.path))
const appName = computed(() => t(app.value.labelKey || 'appConsole'))
const title = computed(() => translateRoute(route.path, route.meta.title || app.value.name))
const summary = computed(() => route.meta.summary || t('moduleReady'))
const cards = computed(() => route.meta.cards || [])
</script>

<template>
  <div class="module-page">
    <section class="page-card module-hero">
      <div>
        <p class="module-kicker">{{ appName }}</p>
        <h1>{{ title }}</h1>
        <p class="module-summary">{{ summary }}</p>
      </div>
    </section>

    <section class="module-grid">
      <article v-for="item in cards" :key="item.title" class="page-card module-card">
        <strong>{{ item.title }}</strong>
        <p>{{ item.description }}</p>
      </article>
    </section>
  </div>
</template>

<style scoped>
.module-page {
  display: grid;
  gap: 18px;
}

.module-hero {
  padding: 28px 32px;
  color: #fff;
  border-radius: 24px;
  background:
    radial-gradient(circle at top right, rgba(125, 211, 252, 0.22), transparent 24%),
    linear-gradient(135deg, #1f2b5a 0%, #3657c9 60%, #4f7dff 100%);
}

.module-kicker {
  margin: 0 0 10px;
  letter-spacing: 0.14em;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.7);
}

.module-hero h1 {
  margin: 0;
  font-size: 34px;
}

.module-summary {
  margin: 14px 0 0;
  max-width: 640px;
  line-height: 1.75;
  color: rgba(255, 255, 255, 0.86);
}

.module-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.module-card {
  min-height: 150px;
}

.module-card strong {
  font-size: 18px;
  color: #0f172a;
}

.module-card p {
  margin: 12px 0 0;
  color: #64748b;
  line-height: 1.7;
}

@media (max-width: 960px) {
  .module-grid {
    grid-template-columns: 1fr;
  }
}
</style>
