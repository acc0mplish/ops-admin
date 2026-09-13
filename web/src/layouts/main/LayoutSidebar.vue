<script setup>
// I5-I (i5-plan §3.8·CW-ML) — MainLayout.vue 1,397행 분할 자식(사이드바).
// 원본 좌표: 템플릿 :520-607·CSS :742-1080·:1248-1295(사멸 app-switcher-menu 블록 —
// 템플릿 미적용, 삭제 없이 byte-bloc 보존). displayTitle은 useLayoutMenus에서 직접
// 임포트하며 자식 템플릿의 displayTitle(menu) 호출이 CW-ML 클레임 대상이다.
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowDown, Check, Grid, House } from '@element-plus/icons-vue'
import { appDefinitions, setCurrentApp } from '../../utils/apps'
import { displayTitle } from '../../composables/useLayoutMenus'
import { t } from '../../utils/i18n-runtime'

defineProps({
  page: {
    type: Object,
    required: true
  }
})

const router = useRouter()
const appNavigationVisible = ref(false)

// 원본 MainLayout.vue:463-470 — 플랫폼 내비게이션 앱 전환(사이드바 전용)
function switchApp(key) {
  const targetApp = appDefinitions.find((item) => item.key === key) || appDefinitions[0]
  setCurrentApp(targetApp.key)
  appNavigationVisible.value = false
  if (router.currentRoute.value.path !== targetApp.defaultRoute) {
    router.push(targetApp.defaultRoute)
  }
}
</script>

<template>
    <el-aside :width="page.collapsed ? '78px' : '200px'" class="layout-aside" :class="{ 'is-page.collapsed': page.collapsed }">
      <div class="brand-box">
        <div class="brand-row">
          <img v-if="page.logoImage" :src="page.logoImage" alt="logo" class="brand-image" />
          <div v-else class="brand-mark">{{ page.currentLogoText() }}</div>
          <div v-if="!page.collapsed" class="brand-text">
            <strong>{{ page.siteName }}</strong>
            <span>{{ page.layoutConfig.siteSlogan || t('siteSloganDefault') }}</span>
          </div>
          <el-popover v-model:visible="appNavigationVisible" placement="bottom-start" :width="820" trigger="click" popper-class="platform-nav-popper">
            <template #reference>
              <button type="button" class="platform-nav-trigger" :aria-label="t('appSwitch')" :title="t('appSwitch')">
                <el-icon><Grid /></el-icon>
                <span>{{ t('platformNavigation') }}</span>
                <el-icon class="platform-nav-arrow"><ArrowDown /></el-icon>
              </button>
            </template>
            <div class="platform-navigation">
              <div class="platform-navigation-head">
                <div>
                  <span>PLATFORM NAVIGATION</span>
                  <strong>{{ t('applicationPlatform') }}</strong>
                </div>
                <small>{{ t('applicationPlatformHint') }}</small>
              </div>
              <div class="platform-navigation-grid">
                <button
                  v-for="app in page.appNavigationItems"
                  :key="app.key"
                  type="button"
                  class="platform-app-card"
                  :class="{ active: app.key === page.currentApp.key }"
                  :style="{ '--app-nav-accent': app.accent, '--app-nav-soft': app.softAccent }"
                  @click="switchApp(app.key)"
                >
                  <span class="platform-app-icon"><el-icon><component :is="app.icon || House" /></el-icon></span>
                  <span class="platform-app-copy">
                    <strong>{{ t(app.labelKey || 'appConsole') }}</strong>
                    <small>{{ t('menuEntries', { count: app.menuCount }) }}</small>
                  </span>
                  <el-icon v-if="app.key === page.currentApp.key" class="platform-app-check"><Check /></el-icon>
                </button>
              </div>
            </div>
          </el-popover>
        </div>
      </div>

      <el-scrollbar class="sidebar-scroll">
        <el-menu
          :default-active="page.activePath"
          :collapse="page.collapsed"
          :collapse-transition="false"
          class="sidebar-menu"
          router
        >
          <template v-for="menu in page.sidebarMenus" :key="menu.path || menu.title">
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
</template>

<style scoped>
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
  padding: 16px 9px 12px;
  min-height: 68px;
  overflow: hidden;
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}
.layout-aside.is-collapsed .brand-box {
  min-height: 104px;
}
.layout-aside.is-collapsed .brand-row {
  flex-wrap: wrap;
  justify-content: center;
  gap: 9px;
}
.layout-aside.is-collapsed .platform-nav-trigger {
  margin-left: 0;
}

.brand-mark,
.brand-image {
  width: 38px;
  height: 38px;
  min-width: 38px;
  max-width: 38px;
  min-height: 38px;
  max-height: 38px;
  border-radius: 11px;
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
  font-size: 16px;
  white-space: nowrap;
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

.platform-nav-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  margin-left: auto;
  padding: 0;
  border: 1px solid rgba(177, 197, 255, 0.58);
  border-radius: 12px;
  color: #fff;
  background: linear-gradient(135deg, rgba(112, 134, 255, 0.78), rgba(91, 108, 249, 0.48));
  box-shadow: 0 0 0 3px rgba(130, 151, 255, 0.14), 0 8px 18px rgba(6, 16, 66, 0.28);
  cursor: pointer;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  transition: transform 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}
.platform-nav-trigger .el-icon { font-size: 19px; }
.platform-nav-trigger > span,
.platform-nav-trigger .platform-nav-arrow { display: none; }
.platform-nav-trigger:hover {
  color: #fff;
  border-color: #fff;
  background: linear-gradient(135deg, #7c8eff, #596cf4);
  box-shadow: 0 0 0 4px rgba(141, 160, 255, 0.22), 0 12px 24px rgba(6, 16, 66, 0.34);
  transform: translateY(-1px);
}
.platform-nav-arrow { font-size: 11px; }
.platform-navigation { padding: 16px; }
.platform-navigation-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18px;
  margin: 2px 2px 14px;
}
.platform-navigation-head span,
.platform-navigation-head strong { display: block; }
.platform-navigation-head span {
  color: #8493ab;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
}
.platform-navigation-head strong {
  margin-top: 4px;
  color: #172b4d;
  font-size: 18px;
}
.platform-navigation-head small {
  color: #7a8aa3;
  font-size: 12px;
}
.platform-navigation-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  padding: 12px;
  border: 1px solid #e3eaf5;
  border-radius: 16px;
  background: linear-gradient(145deg, #fbfdff, #f4f8fd);
}
.platform-app-card {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 78px;
  gap: 11px;
  padding: 12px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: transparent;
  color: #172b4d;
  cursor: pointer;
  text-align: left;
  transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease;
}
.platform-app-card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--app-nav-accent) 34%, #dce6f3);
  background: #fff;
  box-shadow: 0 8px 18px rgba(52, 78, 117, 0.1);
}
.platform-app-card.active {
  border-color: color-mix(in srgb, var(--app-nav-accent) 45%, #dce6f3);
  background: linear-gradient(110deg, var(--app-nav-soft), rgba(255, 255, 255, 0.94));
  box-shadow: inset 0 0 0 1px var(--app-nav-soft);
}
.platform-app-icon {
  display: grid;
  width: 42px;
  height: 42px;
  min-width: 42px;
  place-items: center;
  border-radius: 12px;
  color: #fff;
  background: var(--app-nav-accent);
  box-shadow: 0 8px 16px color-mix(in srgb, var(--app-nav-accent) 28%, transparent);
  font-size: 21px;
}
.platform-app-copy { min-width: 0; }
.platform-app-copy strong,
.platform-app-copy small { display: block; }
.platform-app-copy strong {
  overflow: hidden;
  color: #1e3559;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.platform-app-copy small {
  margin-top: 5px;
  color: #8190a9;
  font-size: 12px;
}
.platform-app-check {
  position: absolute;
  top: 10px;
  right: 10px;
  color: var(--app-nav-accent);
  font-size: 15px;
}
:global(.platform-nav-popper.el-popper) {
  padding: 0;
  overflow: hidden;
  border: 1px solid #dce6f3;
  border-radius: 18px;
  box-shadow: 0 18px 44px rgba(24, 44, 78, 0.2);
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
  margin: 5px 8px;
  border-radius: 10px;
  color: #d8defb;
}

:deep(.sidebar-menu .el-menu-item .el-icon),
:deep(.sidebar-menu .el-sub-menu__title .el-icon) {
  margin-right: 8px;
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


</style>
