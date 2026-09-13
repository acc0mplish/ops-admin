<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Check, House, Search } from '@element-plus/icons-vue'
import LayoutSidebar from './main/LayoutSidebar.vue'
import LayoutHeader from './main/LayoutHeader.vue'
import { useLayoutMenus, findMenuTitle, flattenMenus, normalizeBackendMenus, normalizeStaticMenus } from '../composables/useLayoutMenus'
import { useLayoutTags } from '../composables/useLayoutTags'
import { logoutSession, profile } from '../api/system'
import {
  getMenus,
  getSidebarCollapsed,
  getUser,
  logout,
  setMenus,
  setPermissions,
  setSidebarCollapsed,
  setUser
} from '../utils/auth'
import { appDefinitions, getAppByRoute, setCurrentApp } from '../utils/apps'
import { t, translateRoute } from '../utils/i18n-runtime'
import { applySystemTheme, getSystemConfig, resolveSystemAsset, setSystemConfig } from '../utils/system-config'

const route = useRoute()
const router = useRouter()
const user = ref(getUser())
const rawMenus = ref(getMenus())
const collapsed = ref(getSidebarCollapsed())
const layoutConfig = ref(getSystemConfig())
const appDrawerVisible = ref(false)
const commandPaletteVisible = ref(false)
const commandKeyword = ref('')
const activePath = computed(() => route.path)
const siteName = computed(() => layoutConfig.value.siteName || 'Ops Admin')
const sidebarTheme = computed(() => layoutConfig.value.sidebarTheme || 'dark')
const currentApp = computed(() => getAppByRoute(route.path))
const currentAppLabel = computed(() => t(currentApp.value.labelKey || 'appConsole'))
const appNavigationItems = computed(() => appDefinitions.map((app) => ({
  ...app,
  menuCount: countAppMenus(app),
  accent: appNavigationAccent[app.key] || '#5b6cf9',
  softAccent: appNavigationSoftAccent[app.key] || 'rgba(91, 108, 249, 0.12)'
})))
const currentTitle = computed(() => translateRoute(route.path, route.meta.title || findMenuTitle(sidebarMenus.value, route.path) || siteName.value))
const logoImage = computed(() => {
  if (layoutConfig.value.logoType === 'upload' || layoutConfig.value.logoType === 'url') {
    return resolveSystemAsset(layoutConfig.value.logoValue)
  }
  return ''
})

// 원본 :60-65 — 메뉴 도메인은 useLayoutMenus로 이동(rawMenus/currentApp 인자 주입)
const { sidebarMenus } = useLayoutMenus({ rawMenus, currentApp })
// 원본 :29·:34-39 — 태그 도메인은 useLayoutTags로 이동
const { tagViews, tagContextMenu, isPinnedTag, ensureTag, handleTagClick, closeTag, openTagContextMenu, closeTagContextMenu, closeLeftTags, closeRightTags, closeOtherTags } = useLayoutTags()

const breadcrumbs = computed(() => buildBreadcrumbs(sidebarMenus.value, route.path, currentTitle.value))
const allCommandItems = computed(() => {
  const seenPaths = new Set()

  return appDefinitions.flatMap((app) => {
    const menus = app.menuSource === 'backend'
      ? normalizeBackendMenus(rawMenus.value)
      : normalizeStaticMenus(app.menus || [])
    const appLabel = t(app.labelKey || 'appConsole')

    return flattenMenus(menus).map((item) => ({
      ...item,
      appKey: app.key,
      appLabel
    }))
  }).filter((item) => {
    if (!item.path || seenPaths.has(item.path)) {
      return false
    }
    seenPaths.add(item.path)
    return true
  })
})

const commandItems = computed(() => {
  const keyword = commandKeyword.value.trim().toLowerCase()
  return allCommandItems.value
    .filter((item) => `${item.appLabel} ${item.parentTitle} ${item.title} ${item.path}`.toLowerCase().includes(keyword))
    .slice(0, 40)
})

const appNavigationAccent = {
  console: '#5b6cf9',
  assets: '#11a765',
  containers: '#0ea5e9',
  ops: '#f59e0b',
  applications: '#3b82f6',
  notify: '#ec4899',
  integration: '#14b8a6',
  monitor: '#6366f1',
  domains: '#8b5cf6'
}

const appNavigationSoftAccent = {
  console: 'rgba(91, 108, 249, 0.13)',
  assets: 'rgba(17, 167, 101, 0.13)',
  containers: 'rgba(14, 165, 233, 0.13)',
  ops: 'rgba(245, 158, 11, 0.14)',
  applications: 'rgba(59, 130, 246, 0.13)',
  notify: 'rgba(236, 72, 153, 0.13)',
  integration: 'rgba(20, 184, 166, 0.13)',
  monitor: 'rgba(99, 102, 241, 0.13)',
  domains: 'rgba(139, 92, 246, 0.13)'
}

function countAppMenus(app) {
  const countTree = (items = []) => items.reduce((total, item) => total + 1 + countTree(item.children), 0)
  return app.menuSource === 'backend' ? countTree(rawMenus.value) : countTree(app.menus)
}

function openCommandPalette() {
  commandKeyword.value = ''
  commandPaletteVisible.value = true
}

function selectCommand(item) {
  commandPaletteVisible.value = false
  router.push(item.path)
}

function onGlobalKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    openCommandPalette()
  }
}

function buildBreadcrumbs(menuList, path, fallbackTitle) {
  if (path === '/dashboard') {
    return [
      { title: currentAppLabel.value, path: '/dashboard' },
      { title: t('dashboard'), path }
    ]
  }

  const trail = findMenuTrail(menuList, path)
  if (trail.length) {
    return [
      { title: currentAppLabel.value, path: currentApp.value.defaultRoute },
      ...trail.map((item) => ({ title: displayTitle(item), path: item.path || path }))
    ]
  }

  return [
    { title: currentAppLabel.value, path: currentApp.value.defaultRoute },
    { title: fallbackTitle, path }
  ]
}

function findMenuTrail(menuList, path, parents = []) {
  for (const item of menuList) {
    const trail = [...parents, item]
    if (item.path === path) {
      return trail
    }
    if (item.children?.length) {
      const childTrail = findMenuTrail(item.children, path, trail)
      if (childTrail.length) {
        return childTrail
      }
    }
  }
  return []
}

function currentLogoText() {
  if (layoutConfig.value.logoType === 'text' && layoutConfig.value.logoValue) {
    return String(layoutConfig.value.logoValue).slice(0, 2).toUpperCase()
  }
  return 'OA'
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value
  setSidebarCollapsed(collapsed.value)
}

function handleLogout() {
  logoutSession().catch(() => {})
  logout()
  router.push('/login')
}

function openProfile() {
  router.push('/profile')
}

async function syncProfile() {
  try {
    const data = await profile()
    if (data?.user) {
      setUser(data.user)
      user.value = data.user
    }
    if (data?.leftMenuList) {
      setMenus(data.leftMenuList)
      rawMenus.value = data.leftMenuList
    }
    if (data?.permissionList) {
      setPermissions(data.permissionList)
    }
    if (data?.systemConfig) {
      setSystemConfig(data.systemConfig)
      layoutConfig.value = data.systemConfig
      applySystemTheme(data.systemConfig)
    }
  } catch {
    rawMenus.value = getMenus()
    user.value = getUser()
    layoutConfig.value = getSystemConfig()
    applySystemTheme(layoutConfig.value)
  }
}

// I5-I 자식 주입 번들 — reactive 래핑으로 ref/computed가 언랩되어
// 자식 템플릿의 page.x 재배선이 동작한다(§5 #11).
const page = reactive({
  sidebarMenus,
  collapsed,
  siteName,
  layoutConfig,
  logoImage,
  currentLogoText,
  appNavigationItems,
  currentApp,
  activePath,
  breadcrumbs,
  user,
  toggleCollapsed,
  openCommandPalette,
  openProfile,
  handleLogout,
  tagViews,
  tagContextMenu,
  isPinnedTag,
  handleTagClick,
  closeTag,
  openTagContextMenu,
  closeTagContextMenu,
  closeLeftTags,
  closeRightTags,
  closeOtherTags
})

watch(
  () => route.fullPath,
  () => {
    closeTagContextMenu()
    setCurrentApp(currentApp.value.key)
    ensureTag(route.path, currentTitle.value, route.fullPath)
  },
  { immediate: true }
)

onMounted(() => {
  applySystemTheme(layoutConfig.value)
  syncProfile()
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))
</script>

<template>
  <el-container class="layout-shell" :class="[`theme-${sidebarTheme}`]">
    <LayoutSidebar :page="page" />

    <el-container class="main-shell">
      <LayoutHeader :page="page" />

      <el-main class="layout-main">
        <router-view v-slot="{ Component, route: currentRoute }">
          <keep-alive>
            <component :is="Component" v-if="currentRoute.meta.keepAlive" :key="currentRoute.fullPath" />
          </keep-alive>
          <component :is="Component" v-if="!currentRoute.meta.keepAlive" />
        </router-view>
      </el-main>
    </el-container>

    <el-drawer v-if="false" v-model="appDrawerVisible" :size="320" direction="ltr" class="app-drawer" :with-header="false">
      <div class="app-drawer-header">
        <span>{{ t('appSwitch') }}</span>
        <strong>{{ currentAppLabel }}</strong>
      </div>

      <div class="app-drawer-list">
        <button
          v-for="app in appDefinitions"
          :key="app.key"
          type="button"
          class="app-drawer-item"
          :class="{ active: app.key === currentApp.key }"
          @click="switchApp(app.key)"
        >
          <span class="app-drawer-icon">
            <el-icon><component :is="app.icon || House" /></el-icon>
          </span>
          <span class="app-drawer-text">
            <strong>{{ t(app.labelKey || 'appConsole') }}</strong>
            <small>{{ app.key === currentApp.key ? t('currentApplication') : t('switchApplication') }}</small>
          </span>
          <el-icon v-if="app.key === currentApp.key" class="app-drawer-check"><Check /></el-icon>
        </button>
      </div>
    </el-drawer>

    <el-dialog v-model="commandPaletteVisible" class="command-palette" width="640px" :show-close="false" :append-to-body="true">
      <template #header>
        <div class="command-palette-head">
          <div>
            <span>GLOBAL COMMAND</span>
            <strong>{{ t('quickJumpSearch') }}</strong>
          </div>
          <kbd>ESC</kbd>
        </div>
      </template>
      <el-input v-model="commandKeyword" autofocus :placeholder="t('searchPagesPlaceholder')" clearable>
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="command-result-list">
        <button v-for="item in commandItems" :key="`${item.appKey}:${item.path}`" type="button" class="command-result" @click="selectCommand(item)">
          <el-icon><component :is="item.icon" /></el-icon>
          <span><b>{{ item.title }}</b><small>{{ [item.appLabel, item.parentTitle].filter(Boolean).join(' / ') }} · {{ item.path }}</small></span>
          <em>Enter</em>
        </button>
        <el-empty v-if="!commandItems.length" :description="t('noMatchingPage')" :image-size="48" />
      </div>
    </el-dialog>
  </el-container>
</template>

<style scoped>
.layout-shell {
  min-height: 100vh;
  background: #eef1f8;
}

.main-shell {
  min-width: 0;
}

.layout-main {
  padding: 20px;
}

:deep(.app-drawer .el-drawer__body) {
  padding: 0;
  background: #f7f9fc;
}

.app-drawer-header {
  padding: 22px 22px 18px;
  border-bottom: 1px solid #e6ebf3;
  background: #fff;
}

.app-drawer-header span {
  display: block;
  margin-bottom: 8px;
  font-size: 13px;
  color: #64748b;
}

.app-drawer-header strong {
  font-size: 22px;
  color: #0f172a;
}

.app-drawer-list {
  display: grid;
  gap: 10px;
  padding: 16px;
}

.app-drawer-item {
  width: 100%;
  min-height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #fff;
  color: #0f172a;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
}

.app-drawer-item:hover {
  border-color: #cbd5e1;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.08);
}

.app-drawer-item.active {
  border-color: rgba(91, 108, 249, 0.42);
  background: linear-gradient(90deg, rgba(91, 108, 249, 0.12), rgba(91, 108, 249, 0.04));
}

.app-drawer-icon {
  width: 38px;
  height: 38px;
  min-width: 38px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  color: var(--app-primary);
  background: rgba(91, 108, 249, 0.1);
  font-size: 18px;
}

.app-drawer-text {
  flex: 1;
  display: grid;
  gap: 4px;
  min-width: 0;
}

.app-drawer-text strong {
  font-size: 15px;
  color: #0f172a;
}

.app-drawer-text small {
  font-size: 12px;
  color: #64748b;
}

.app-drawer-check {
  color: var(--app-primary);
}

@media (max-width: 960px) {
  .layout-header {
    padding: 12px;
  }

  .layout-main {
    padding: 14px;
  }
}
:global(.command-palette.el-dialog) { overflow: hidden; border: 1px solid #dce6f3; border-top: 2px solid var(--app-primary); border-radius: 12px; background: #fff; box-shadow: 0 26px 70px rgba(24, 44, 78, .22); }
:global(.command-palette .el-dialog__header) { margin: 0; padding: 16px 18px 12px; }
:global(.command-palette .el-dialog__body) { padding: 0 18px 18px; }
.command-palette-head { display: flex; align-items: center; justify-content: space-between; }.command-palette-head span, .command-palette-head strong { display: block; }.command-palette-head span { color: var(--app-primary); font-size: 10px; font-weight: 800; letter-spacing: .11em; }.command-palette-head strong { margin-top: 4px; color: #172b4d; font-size: 17px; }
.command-result-list { display: grid; gap: 6px; max-height: 380px; margin-top: 12px; overflow: auto; }.command-result { display: flex; align-items: center; gap: 11px; width: 100%; padding: 11px 12px; border: 1px solid transparent; border-radius: 8px; background: #f7f9fd; color: #40516d; text-align: left; cursor: pointer; }.command-result:hover { border-color: #c6d9ee; background: #eef4fc; }.command-result > span { display: grid; gap: 2px; min-width: 0; }.command-result b { color: #172b4d; font-size: 13px; }.command-result small { overflow: hidden; color: #8190a9; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.command-result em { margin-left: auto; }
@media (max-width: 960px) {
  .layout-main {
    padding: 14px;
  }
}
</style>