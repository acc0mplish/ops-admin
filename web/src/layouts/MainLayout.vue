<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, Check, Expand, Fold, House, Switch } from '@element-plus/icons-vue'
import { profile } from '../api/system'
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
import { locale, setLocale, t, translateRoute } from '../utils/i18n'
import { applySystemTheme, getSystemConfig, resolveSystemAsset, setSystemConfig } from '../utils/system-config'

const route = useRoute()
const router = useRouter()
const TAGS_KEY = 'ops-admin-tags-view'

const user = ref(getUser())
const rawMenus = ref(getMenus())
const collapsed = ref(getSidebarCollapsed())
const layoutConfig = ref(getSystemConfig())
const tagViews = ref(loadTags())
const appDrawerVisible = ref(false)
const tagContextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  tag: null
})

const currentLocale = computed(() => locale.value)
const activePath = computed(() => route.path)
const siteName = computed(() => layoutConfig.value.siteName || 'Ops Admin')
const sidebarTheme = computed(() => layoutConfig.value.sidebarTheme || 'dark')
const currentApp = computed(() => getAppByRoute(route.path))
const currentAppLabel = computed(() => t(currentApp.value.labelKey || 'appConsole'))
const currentTitle = computed(() => translateRoute(route.path, route.meta.title || findMenuTitle(sidebarMenus.value, route.path) || siteName.value))
const logoImage = computed(() => {
  if (layoutConfig.value.logoType === 'upload' || layoutConfig.value.logoType === 'url') {
    return resolveSystemAsset(layoutConfig.value.logoValue)
  }
  return ''
})

const sidebarMenus = computed(() => {
  if (currentApp.value.menuSource === 'backend') {
    return normalizeBackendMenus(rawMenus.value)
  }
  return normalizeStaticMenus(currentApp.value.menus || [])
})

const breadcrumbs = computed(() => buildBreadcrumbs(sidebarMenus.value, route.path, currentTitle.value))

function loadTags() {
  const defaults = [{ title: t('dashboard'), path: '/dashboard', affix: true }]
  const raw = localStorage.getItem(TAGS_KEY)
  if (!raw) {
    return defaults
  }
  try {
    const parsed = JSON.parse(raw)
    const source = Array.isArray(parsed) && parsed.length ? parsed : defaults
    return source.map((item) => ({
      ...item,
      title: translateRoute(item.path, item.title),
      affix: isPinnedTag(item)
    }))
  } catch {
    return defaults
  }
}

function saveTags() {
  localStorage.setItem(
    TAGS_KEY,
    JSON.stringify(tagViews.value.map((item) => ({ ...item, affix: isPinnedTag(item) })))
  )
}

function isPinnedTag(tag) {
  return tag?.path === '/dashboard'
}

function normalizeBackendMenus(raw) {
  const menuList = (raw || [])
    .filter((item) => !isAppEntryMenu(item))
    .map((item) => ({
      title: item.menuName,
      path: item.url || '',
      icon: item.icon || 'Menu',
      children: (item.menuSvoList || [])
        .filter((child) => child.menuType !== 3 && !isAppEntryMenu(child))
        .map((child) => ({
          title: child.menuName,
          path: child.url || '',
          icon: child.icon || 'Document',
          children: []
        }))
    }))

  if (!menuList.some((item) => item.path === '/dashboard')) {
    menuList.unshift({
      title: t('dashboard'),
      path: '/dashboard',
      icon: 'House',
      children: []
    })
  }

  return menuList
}

function isAppEntryMenu(item) {
  const name = item.menuName || item.title || ''
  const path = item.url || item.path || ''
  return name === '控制台' || name === 'Console' || path === '/console'
}

function normalizeStaticMenus(raw) {
  return (raw || []).map((item) => ({
    ...item,
    children: normalizeStaticMenus(item.children || [])
  }))
}

function findMenuTitle(menuList, path) {
  for (const item of menuList) {
    if (item.path === path) {
      return item.title
    }
    if (item.children?.length) {
      const match = item.children.find((child) => child.path === path)
      if (match) {
        return match.title
      }
    }
  }
  return ''
}

function displayTitle(item) {
  return translateRoute(item.path, item.titleKey ? t(item.titleKey) : item.title)
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

function ensureTag(path, title) {
  if (!path || path === '/login') {
    return
  }
  const translated = translateRoute(path, title || '未命名页面')
  const found = tagViews.value.find((item) => item.path === path)
  if (found) {
    found.title = translated
    saveTags()
    return
  }
  tagViews.value.push({
    title: translated,
    path,
    affix: path === '/dashboard'
  })
  saveTags()
}

function handleTagClick(path) {
  if (path && path !== route.path) {
    router.push(path)
  }
}

function closeTag(tag) {
  if (isPinnedTag(tag)) {
    return
  }
  const index = tagViews.value.findIndex((item) => item.path === tag.path)
  if (index === -1) {
    return
  }
  const wasActive = route.path === tag.path
  tagViews.value.splice(index, 1)
  saveTags()
  if (wasActive) {
    const nextTag = tagViews.value[index - 1] || tagViews.value[index] || tagViews.value[0]
    router.push(nextTag?.path || '/dashboard')
  }
}

function openTagContextMenu(event, tag) {
  event.preventDefault()
  tagContextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
    tag
  }
}

function closeTagContextMenu() {
  tagContextMenu.value.visible = false
}

function closeLeftTags() {
  const target = tagContextMenu.value.tag
  if (!target) {
    return
  }
  const targetIndex = tagViews.value.findIndex((item) => item.path === target.path)
  if (targetIndex <= 0) {
    closeTagContextMenu()
    return
  }
  const removedActive = tagViews.value.some((item, index) => index < targetIndex && !isPinnedTag(item) && item.path === route.path)
  tagViews.value = tagViews.value.filter((item, index) => index >= targetIndex || isPinnedTag(item))
  saveTags()
  closeTagContextMenu()
  if (removedActive) {
    router.push(target.path)
  }
}

function closeRightTags() {
  const target = tagContextMenu.value.tag
  if (!target) {
    return
  }
  const targetIndex = tagViews.value.findIndex((item) => item.path === target.path)
  if (targetIndex === -1) {
    closeTagContextMenu()
    return
  }
  const removedActive = tagViews.value.some((item, index) => index > targetIndex && !isPinnedTag(item) && item.path === route.path)
  tagViews.value = tagViews.value.filter((item, index) => index <= targetIndex || isPinnedTag(item))
  saveTags()
  closeTagContextMenu()
  if (removedActive) {
    router.push(target.path)
  }
}

function closeOtherTags() {
  const target = tagContextMenu.value.tag
  if (!target) {
    return
  }
  const keepPath = target.path
  tagViews.value = tagViews.value.filter((item) => isPinnedTag(item) || item.path === keepPath)
  saveTags()
  closeTagContextMenu()
  if (route.path !== keepPath && !tagViews.value.some((item) => item.path === route.path)) {
    router.push(keepPath)
  }
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value
  setSidebarCollapsed(collapsed.value)
}

function handleLogout() {
  logout()
  router.push('/login')
}

function openProfile() {
  router.push('/profile')
}

function switchLocale(value) {
  setLocale(value)
  tagViews.value = tagViews.value.map((item) => ({
    ...item,
    title: translateRoute(item.path, item.title)
  }))
  saveTags()
}

function switchApp(key) {
  const targetApp = appDefinitions.find((item) => item.key === key) || appDefinitions[0]
  setCurrentApp(targetApp.key)
  appDrawerVisible.value = false
  if (route.path !== targetApp.defaultRoute) {
    router.push(targetApp.defaultRoute)
  }
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

watch(
  () => route.path,
  () => {
    closeTagContextMenu()
    setCurrentApp(currentApp.value.key)
    ensureTag(route.path, currentTitle.value)
  },
  { immediate: true }
)

watch(
  () => locale.value,
  () => {
    tagViews.value = tagViews.value.map((item) => ({
      ...item,
      title: translateRoute(item.path, item.title)
    }))
    saveTags()
  }
)

onMounted(() => {
  applySystemTheme(layoutConfig.value)
  syncProfile()
})
</script>

<template>
  <el-container class="layout-shell" :class="[`theme-${sidebarTheme}`]">
    <el-aside :width="collapsed ? '78px' : '236px'" class="layout-aside">
      <div class="brand-box">
        <div class="brand-row">
          <img v-if="logoImage" :src="logoImage" alt="logo" class="brand-image" />
          <div v-else class="brand-mark">{{ currentLogoText() }}</div>
          <div v-if="!collapsed" class="brand-text">
            <strong>{{ siteName }}</strong>
            <span>{{ layoutConfig.siteSlogan || '个人运维管理平台' }}</span>
          </div>
          <el-tooltip v-if="!collapsed" :content="t('appSwitch')" placement="right">
            <button type="button" class="brand-switch-button" @click="appDrawerVisible = true">
              <el-icon><Switch /></el-icon>
            </button>
          </el-tooltip>
        </div>
      </div>

      <el-scrollbar class="sidebar-scroll">
        <el-menu
          :default-active="activePath"
          :collapse="collapsed"
          :collapse-transition="false"
          class="sidebar-menu"
          router
        >
          <template v-for="menu in sidebarMenus" :key="menu.path || menu.title">
            <el-menu-item v-if="!menu.children?.length" :index="menu.path">
              <el-icon><component :is="menu.icon || House" /></el-icon>
              <template #title>{{ displayTitle(menu) }}</template>
            </el-menu-item>

            <el-sub-menu v-else :index="menu.path || menu.title">
              <template #title>
                <el-icon><component :is="menu.icon || 'Menu'" /></el-icon>
                <span>{{ displayTitle(menu) }}</span>
              </template>
              <template v-for="child in menu.children" :key="child.path || child.title">
                <el-menu-item v-if="!child.children?.length" :index="child.path">
                  <el-icon><component :is="child.icon || 'Document'" /></el-icon>
                  <template #title>{{ displayTitle(child) }}</template>
                </el-menu-item>
                <el-sub-menu v-else :index="child.path || child.title">
                  <template #title>
                    <el-icon><component :is="child.icon || 'Document'" /></el-icon>
                    <span>{{ displayTitle(child) }}</span>
                  </template>
                  <el-menu-item v-for="grandChild in child.children" :key="grandChild.path" :index="grandChild.path">
                    <el-icon><component :is="grandChild.icon || 'Document'" /></el-icon>
                    <template #title>{{ displayTitle(grandChild) }}</template>
                  </el-menu-item>
                </el-sub-menu>
              </template>
            </el-sub-menu>
          </template>
        </el-menu>
      </el-scrollbar>
    </el-aside>

    <el-container class="main-shell">
      <el-header class="layout-header">
        <div class="header-top">
          <div class="header-left">
            <el-button class="collapse-button" circle @click="toggleCollapsed">
              <el-icon><component :is="collapsed ? Expand : Fold" /></el-icon>
            </el-button>

            <el-breadcrumb separator="/">
              <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
                {{ item.title }}
              </el-breadcrumb-item>
            </el-breadcrumb>
          </div>

          <div class="header-right">
            <el-dropdown trigger="click" @command="switchLocale">
              <div class="locale-box">
                <span>{{ currentLocale === 'zh-CN' ? 'CN' : 'EN' }}</span>
                <el-icon><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="zh-CN">
                    <div class="locale-item">
                      <span>CN</span>
                      <el-icon v-if="currentLocale === 'zh-CN'"><Check /></el-icon>
                    </div>
                  </el-dropdown-item>
                  <el-dropdown-item command="en-US">
                    <div class="locale-item">
                      <span>EN</span>
                      <el-icon v-if="currentLocale === 'en-US'"><Check /></el-icon>
                    </div>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>

            <el-dropdown>
              <div class="user-box">
                <div class="user-avatar">{{ (user.nickname || user.username || 'A').slice(0, 1).toUpperCase() }}</div>
                <span>{{ user.nickname || user.username || 'admin' }}</span>
                <el-icon><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openProfile">{{ t('profile') }}</el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">{{ t('logout') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>

        <div class="tabs-bar">
          <div
            v-for="tag in tagViews"
            :key="tag.path"
            class="tab-chip"
            :class="{ active: tag.path === route.path }"
            @click="handleTagClick(tag.path)"
            @contextmenu="openTagContextMenu($event, tag)"
          >
            <span>{{ tag.title }}</span>
            <button v-if="!isPinnedTag(tag)" type="button" class="tab-close" @click.stop="closeTag(tag)">×</button>
          </div>
        </div>
      </el-header>

      <el-main class="layout-main">
        <router-view />
      </el-main>
    </el-container>

    <el-drawer v-model="appDrawerVisible" :size="320" direction="ltr" class="app-drawer" :with-header="false">
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
            <small>{{ app.key === currentApp.key ? '当前应用' : '切换进入' }}</small>
          </span>
          <el-icon v-if="app.key === currentApp.key" class="app-drawer-check"><Check /></el-icon>
        </button>
      </div>
    </el-drawer>

    <div v-if="tagContextMenu.visible" class="tag-menu-mask" @click="closeTagContextMenu">
      <div
        class="tag-context-menu"
        :style="{ left: `${tagContextMenu.x}px`, top: `${tagContextMenu.y}px` }"
        @click.stop
      >
        <button type="button" @click="closeLeftTags">关闭左侧标签页</button>
        <button type="button" @click="closeRightTags">关闭右侧标签页</button>
        <button type="button" @click="closeOtherTags">关闭其他标签页</button>
      </div>
    </div>
  </el-container>
</template>

<style scoped>
.layout-shell {
  min-height: 100vh;
  background: #eef1f8;
}

.layout-aside {
  background: linear-gradient(180deg, #20295b 0%, #1f2552 100%);
  color: #d8defb;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  transition: width 0.2s ease;
}

.theme-light .layout-aside {
  background: linear-gradient(180deg, #30406b 0%, #2d3a61 100%);
}

.brand-box {
  padding: 18px 12px 14px;
  min-height: 74px;
  overflow: hidden;
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.brand-mark,
.brand-image {
  width: 42px;
  height: 42px;
  min-width: 42px;
  max-width: 42px;
  min-height: 42px;
  max-height: 42px;
  border-radius: 12px;
  box-shadow: 0 10px 22px rgba(35, 178, 255, 0.28);
}

.brand-mark {
  display: grid;
  place-items: center;
  font-weight: 800;
  color: #fff;
  background: linear-gradient(135deg, var(--app-primary), #23b2ff);
}

.brand-image {
  object-fit: cover;
  display: block;
}

.brand-text {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.brand-text strong {
  color: #fff;
  font-size: 18px;
}

.brand-text span {
  color: rgba(216, 222, 251, 0.72);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.brand-switch-button {
  width: 30px;
  height: 30px;
  min-width: 30px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 8px;
  color: #d8defb;
  background: rgba(255, 255, 255, 0.08);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.brand-switch-button:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.14);
}

.app-switcher-wrap {
  width: 100%;
  margin-bottom: 20px;
}

.app-switcher,
:deep(.app-switcher .el-tooltip__trigger) {
  width: 100%;
  display: block;
}

.app-switcher-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 48px;
  width: 100%;
  box-sizing: border-box;
  padding: 0 14px;
  border-radius: 11px;
  background: rgba(255, 255, 255, 0.11);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #fff;
  cursor: pointer;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  transition: background 0.2s ease, border-color 0.2s ease;
}

.app-switcher-trigger:hover {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.14);
}

.app-switcher-current {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.app-switcher-current span {
  font-size: 14px;
  font-weight: 600;
  line-height: 1;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-switcher-icon {
  width: 28px;
  height: 28px;
  min-width: 28px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.08);
  font-size: 16px;
}

.app-switcher-arrow {
  color: rgba(255, 255, 255, 0.76);
}

.sidebar-scroll {
  height: calc(100vh - 74px);
}

:deep(.sidebar-menu) {
  border-right: none;
  background: transparent;
}

:deep(.sidebar-menu .el-menu) {
  background: transparent;
}

:deep(.sidebar-menu .el-menu-item),
:deep(.sidebar-menu .el-sub-menu__title) {
  margin: 6px 12px;
  border-radius: 12px;
  color: #d8defb;
}

:deep(.sidebar-menu .el-menu-item:hover),
:deep(.sidebar-menu .el-sub-menu__title:hover) {
  background: rgba(255, 255, 255, 0.08);
}

:deep(.sidebar-menu .el-menu-item.is-active) {
  color: #fff;
  background: linear-gradient(90deg, rgba(91, 108, 249, 0.95), rgba(122, 84, 255, 0.9));
  box-shadow: 0 10px 24px rgba(91, 108, 249, 0.28);
}

:deep(.sidebar-menu > .el-menu-item:first-child),
:deep(.sidebar-menu > .el-sub-menu:first-child .el-sub-menu__title) {
  margin-top: 0;
}

.main-shell {
  min-width: 0;
}

.layout-header {
  height: auto;
  padding: 14px 20px 10px;
  background: rgba(255, 255, 255, 0.94);
  border-bottom: 1px solid #e5eaf4;
  backdrop-filter: blur(10px);
}

.header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.header-left,
.header-right {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.collapse-button {
  border-color: #d9e0ef;
  color: #334155;
}

.locale-box {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 58px;
  height: 32px;
  justify-content: center;
  padding: 0 8px;
  border-radius: 999px;
  border: 1px solid #d8e1f0;
  background: #fff;
  color: #0f172a;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.locale-box:hover {
  background: #f3f6fd;
}

.locale-item {
  min-width: 72px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.user-box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 999px;
  cursor: pointer;
  color: #111827;
}

.user-box:hover {
  background: #f3f6fd;
}

.user-avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(135deg, #1d8cf8, #5b6cf9);
  font-size: 13px;
  font-weight: 700;
}

.tabs-bar {
  display: flex;
  gap: 8px;
  overflow: auto;
  padding-top: 12px;
}

.tab-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  border-radius: 10px;
  border: 1px solid #d8e1f0;
  background: #fff;
  color: #52607a;
  cursor: pointer;
  white-space: nowrap;
}

.tab-chip.active {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(90deg, var(--app-primary), #7c5cff);
}

.tab-close {
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 0;
}

.tag-menu-mask {
  position: fixed;
  inset: 0;
  z-index: 3000;
  background: transparent;
}

.tag-context-menu {
  position: fixed;
  min-width: 156px;
  padding: 6px;
  border: 1px solid #e5eaf4;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.14);
}

.tag-context-menu button {
  width: 100%;
  height: 34px;
  display: flex;
  align-items: center;
  padding: 0 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  text-align: left;
}

.tag-context-menu button:hover {
  color: var(--app-primary);
  background: #f4f7ff;
}

.layout-main {
  padding: 20px;
}

:deep(.app-switcher-menu .el-dropdown-menu__item) {
  min-width: 224px;
  padding: 0;
  line-height: normal;
}

:deep(.app-switcher-menu .el-dropdown-menu__item.is-current-app) {
  background: linear-gradient(90deg, rgba(91, 108, 249, 0.12), rgba(122, 84, 255, 0.08));
}

:deep(.app-switcher-menu.el-dropdown-menu) {
  padding: 10px;
  border: 1px solid #e7ebf4;
  border-radius: 14px;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.16);
}

.app-option {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 12px;
}

.app-option-main {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-option-icon {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  color: var(--app-primary);
  background: rgba(91, 108, 249, 0.08);
}

:deep(.app-switcher-menu .el-dropdown-menu__item:not(.is-disabled):focus) .app-option,
:deep(.app-switcher-menu .el-dropdown-menu__item:not(.is-disabled):hover) .app-option {
  background: #f7f9ff;
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
</style>
