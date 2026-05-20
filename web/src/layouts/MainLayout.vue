<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, Fold, Expand, House, Check } from '@element-plus/icons-vue'
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
import { applySystemTheme, getSystemConfig, resolveSystemAsset, setSystemConfig } from '../utils/system-config'
import { locale, setLocale, t, translateRoute } from '../utils/i18n'

const route = useRoute()
const router = useRouter()
const TAGS_KEY = 'ops-admin-tags-view'

const user = ref(getUser())
const rawMenus = ref(getMenus())
const collapsed = ref(getSidebarCollapsed())
const layoutConfig = ref(getSystemConfig())
const tagViews = ref(loadTags())

const currentLocale = computed(() => locale.value)
const sidebarMenus = computed(() => normalizeMenus(rawMenus.value))
const activePath = computed(() => route.path)
const siteName = computed(() => layoutConfig.value.siteName || 'Ops Admin')
const sidebarTheme = computed(() => layoutConfig.value.sidebarTheme || 'dark')
const logoImage = computed(() => {
  if (layoutConfig.value.logoType === 'upload' || layoutConfig.value.logoType === 'url') {
    return resolveSystemAsset(layoutConfig.value.logoValue)
  }
  return ''
})
const currentTitle = computed(() => translateRoute(route.path, route.meta.title || findMenuTitle(sidebarMenus.value, route.path) || 'Ops Admin'))
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
    return source.map((item) => ({ ...item, title: translateRoute(item.path, item.title) }))
  } catch {
    return defaults
  }
}

function saveTags() {
  localStorage.setItem(TAGS_KEY, JSON.stringify(tagViews.value))
}

function normalizeMenus(raw) {
  const menuList = (raw || []).map((item) => ({
    title: item.menuName,
    path: item.url || '',
    icon: item.icon || 'Menu',
    children: (item.menuSvoList || []).map((child) => ({
      title: child.menuName,
      path: child.url || '',
      icon: child.icon || 'Document'
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

function displayTitle(path, fallback) {
  return translateRoute(path, fallback)
}

function buildBreadcrumbs(menuList, path, fallbackTitle) {
  if (path === '/dashboard') {
    return [{ title: t('dashboard'), path }]
  }
  if (path === '/profile') {
    return [
      { title: t('userCenter'), path: '/profile' },
      { title: t('profile'), path }
    ]
  }

  for (const item of menuList) {
    if (item.path === path) {
      return [{ title: displayTitle(item.path, item.title), path: item.path }]
    }
    if (item.children?.length) {
      const child = item.children.find((entry) => entry.path === path)
      if (child) {
        return [
          { title: displayTitle(item.path, item.title), path: item.path || path },
          { title: displayTitle(child.path, child.title), path: child.path }
        ]
      }
    }
  }
  return [{ title: fallbackTitle, path }]
}

function currentLogoText() {
  if (layoutConfig.value.logoType === 'text' && layoutConfig.value.logoValue) {
    return String(layoutConfig.value.logoValue).slice(0, 2).toUpperCase()
  }
  return 'OA'
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

function toggleCollapsed() {
  collapsed.value = !collapsed.value
  setSidebarCollapsed(collapsed.value)
}

function handleTagClick(path) {
  if (path && path !== route.path) {
    router.push(path)
  }
}

function closeTag(tag) {
  if (tag.affix) {
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
    affix: route.meta.affix === true
  })
  saveTags()
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
  () => ensureTag(route.path, currentTitle.value),
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
    <el-aside :width="collapsed ? '76px' : '220px'" class="layout-aside">
      <div class="brand-box">
        <img v-if="logoImage" :src="logoImage" alt="logo" class="brand-image" />
        <div v-else class="brand-mark">{{ currentLogoText() }}</div>
        <div v-if="!collapsed" class="brand-text">
          <strong>{{ siteName }}</strong>
          <span>{{ layoutConfig.siteSlogan || '运维管理平台' }}</span>
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
              <template #title>{{ displayTitle(menu.path, menu.title) }}</template>
            </el-menu-item>

            <el-sub-menu v-else :index="menu.path || menu.title">
              <template #title>
                <el-icon><component :is="menu.icon || 'Menu'" /></el-icon>
                <span>{{ displayTitle(menu.path, menu.title) }}</span>
              </template>
              <el-menu-item v-for="child in menu.children" :key="child.path" :index="child.path">
                <el-icon><component :is="child.icon || 'Document'" /></el-icon>
                <template #title>{{ displayTitle(child.path, child.title) }}</template>
              </el-menu-item>
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
              <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">{{ item.title }}</el-breadcrumb-item>
            </el-breadcrumb>
          </div>

          <div class="header-right">
            <el-dropdown trigger="click" @command="switchLocale">
              <div class="user-box locale-box">
                <div class="locale-avatar">语</div>
                <span>{{ currentLocale === 'zh-CN' ? 'CN' : 'EN' }}</span>
                <el-icon><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="zh-CN">
                    <div class="locale-item">
                      <span>中文</span>
                      <el-icon v-if="currentLocale === 'zh-CN'"><Check /></el-icon>
                    </div>
                  </el-dropdown-item>
                  <el-dropdown-item command="en-US">
                    <div class="locale-item">
                      <span>English</span>
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
          >
            <span>{{ tag.title }}</span>
            <button v-if="!tag.affix" type="button" class="tab-close" @click.stop="closeTag(tag)">×</button>
          </div>
        </div>
      </el-header>

      <el-main class="layout-main">
        <router-view />
      </el-main>
    </el-container>
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
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 18px 14px;
  min-height: 72px;
  overflow: hidden;
}

.brand-mark,
.brand-image {
  width: 40px;
  height: 40px;
  min-width: 40px;
  max-width: 40px;
  min-height: 40px;
  max-height: 40px;
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

.sidebar-scroll {
  height: calc(100vh - 72px);
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

.locale-item {
  min-width: 92px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.locale-box {
  gap: 6px;
  padding: 5px 8px;
  border: 1px solid #d9e0ef;
  background: #fff;
  font-size: 12px;
}

.locale-avatar {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6, #0ea5e9);
  font-size: 10px;
  font-weight: 700;
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

.layout-main {
  padding: 20px;
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
