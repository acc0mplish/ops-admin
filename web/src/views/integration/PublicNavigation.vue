<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { queryPublicNavigation } from '../../api/integration'
import { it } from '../../utils/integration-i18n'

const route = useRoute()
const loading = ref(true)
const data = ref({ name: '', description: '', items: [] })
const keyword = ref('')
const unavailable = ref(false)

const filteredItems = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  if (!value) return data.value.items || []
  return (data.value.items || []).filter((item) => `${item.name} ${item.description} ${item.url}`.toLowerCase().includes(value))
})
function iconText(name) { return String(name || 'N').trim().slice(0, 1).toUpperCase() }
function displayHost(value) { try { return new URL(value).host } catch { return value || '-' } }
function toneClass(item) { return `tone-${Math.abs(Number(item?.id) || 0) % 5}` }
function openItem(item) { if (item.openMode === 'current') window.location.href = item.url; else { const target = window.open(item.url, '_blank', 'noopener,noreferrer'); if (target) target.opener = null } }

onMounted(async () => {
  try { data.value = await queryPublicNavigation(route.params.token); document.title = `${data.value.name} - ${it('publicNavigationSuffix')}` }
  catch { unavailable.value = true }
  finally { loading.value = false }
})
</script>

<template>
  <main class="public-page">
    <div v-if="loading" class="public-state">{{ it('loadingNavigation') }}</div>
    <div v-else-if="unavailable" class="public-state"><div class="state-icon">!</div><h1>{{ it('publicNavigationUnavailable') }}</h1><p>{{ it('publicNavigationUnavailableDesc') }}</p></div>
    <template v-else>
      <header class="public-header"><div class="header-inner">
        <div class="brand"><span>OA</span><div><strong>Ops Admin</strong><small>{{ it('integrationPublicNavigation') }}</small></div></div>
        <div class="public-title"><span>SHARED NAVIGATION</span><h1>{{ data.name }}</h1><p>{{ data.description || it('commonSystems') }}</p></div>
        <div class="search-box"><el-input v-model="keyword" clearable size="large" :placeholder="it('searchSystemToolAddress')" /><span>{{ it('entryCount', { count: filteredItems.length }) }}</span></div>
      </div></header>
      <section class="public-content">
        <div class="result-meta"><div><strong>{{ it('allNavigation') }}</strong><small>{{ it('openConfiguredWindow') }}</small></div><span>{{ it('entryCount', { count: filteredItems.length }) }}</span></div>
        <div class="public-grid"><button v-for="item in filteredItems" :key="item.id" type="button" class="public-card" :class="toneClass(item)" @click="openItem(item)"><span class="card-top"><span class="public-icon" :class="toneClass(item)"><img v-if="item.iconUrl" :src="item.iconUrl" alt="" /><b v-else>{{ iconText(item.name) }}</b></span><span class="arrow">↗</span></span><span class="public-copy"><strong>{{ item.name }}</strong><small>{{ item.description || it('noEntryDescription') }}</small></span><span class="card-host">{{ displayHost(item.url) }}</span></button></div>
        <el-empty v-if="!filteredItems.length" :description="it('noMatchingNavigation')" />
      </section>
      <footer>{{ it('providedByIntegrationCenter') }}</footer>
    </template>
  </main>
</template>

<style scoped>
.public-page { min-height: 100vh; color: #14223b; background: #f3f6fb; }.public-header { padding: 22px 28px 28px; border-bottom: 1px solid #dfe7f2; background: #fff; }.header-inner { width: min(1480px, 100%); margin: 0 auto; }.brand { display: flex; align-items: center; gap: 10px; }.brand span { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 7px; color: #fff; background: #3275d9; font-weight: 800; }.brand div { display: flex; flex-direction: column; gap: 2px; }.brand small { color: #8996aa; font-size: 11px; }.public-title { margin: 36px 0 20px; }.public-title > span { color: #3975d3; font-size: 12px; font-weight: 800; }.public-title h1 { margin: 6px 0; font-size: 32px; line-height: 1.2; }.public-title p { margin: 0; color: #71809a; }.search-box { display: flex; width: min(680px, 100%); align-items: center; gap: 14px; }.search-box .el-input { flex: 1; }.search-box > span { color: #8390a5; white-space: nowrap; }.public-content { width: min(1480px, calc(100% - 56px)); margin: 0 auto; padding: 26px 0 48px; }.result-meta { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }.result-meta div { display: flex; align-items: baseline; gap: 12px; }.result-meta small { color: #8b98aa; font-weight: 400; }.result-meta span { color: #8390a5; }.public-grid { display: grid; grid-template-columns: repeat(5, minmax(210px, 1fr)); gap: 13px; }.public-card { display: flex; min-width: 0; min-height: 154px; padding: 15px 16px 13px; border: 1px solid #dee6f1; border-top: 3px solid #5b8def; border-radius: 7px; color: inherit; background: #fff; text-align: left; flex-direction: column; cursor: pointer; transition: transform .18s, border-color .18s, box-shadow .18s; }.public-card:hover { transform: translateY(-2px); border-color: #8db5f2; box-shadow: 0 9px 24px rgba(46, 82, 133, .1); }.card-top { display: flex; align-items: flex-start; justify-content: space-between; }.public-icon { display: grid; place-items: center; width: 44px; height: 44px; flex: 0 0 44px; overflow: hidden; border-radius: 8px; color: #2563c3; background: #eaf2ff; font-size: 19px; }.public-icon img { width: 31px; height: 31px; object-fit: contain; }.public-copy { display: flex; min-width: 0; margin-top: 11px; flex: 1; flex-direction: column; gap: 5px; }.public-copy strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.public-copy small { display: -webkit-box; overflow: hidden; color: #7c899d; line-height: 1.4; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }.card-host { overflow: hidden; color: #9aa5b4; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.arrow { color: #8a9ab2; font-size: 18px; }.tone-1 { border-top-color: #21a675; }.tone-2 { border-top-color: #de8a24; }.tone-3 { border-top-color: #18a3b7; }.tone-4 { border-top-color: #7359c9; }.public-icon.tone-1 { color: #14805b; background: #e5f6ef; }.public-icon.tone-2 { color: #b76c12; background: #fff2df; }.public-icon.tone-3 { color: #127d8c; background: #e5f7fa; }.public-icon.tone-4 { color: #6046b3; background: #eeeafe; }.public-state { display: grid; min-height: 100vh; place-content: center; justify-items: center; color: #697991; text-align: center; }.public-state h1 { margin: 14px 0 4px; color: #16243c; }.state-icon { display: grid; place-items: center; width: 54px; height: 54px; border-radius: 50%; color: #c56a25; background: #fff0df; font-size: 28px; font-weight: 800; }footer { padding: 18px; color: #8b98aa; text-align: center; }@media (max-width: 1350px) { .public-grid { grid-template-columns: repeat(4, minmax(200px, 1fr)); } }@media (max-width: 1050px) { .public-grid { grid-template-columns: repeat(3, minmax(200px, 1fr)); } }@media (max-width: 760px) { .public-header { padding-inline: 20px; }.public-content { width: calc(100% - 32px); }.public-grid { grid-template-columns: 1fr; }.public-title { margin-top: 28px; }.result-meta small { display: none; } }
</style>
