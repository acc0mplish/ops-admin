<script setup>
// I5-I (i5-plan §3.8·CW-ML) — MainLayout.vue 1,397행 분할 자식(헤더+태그바).
// 원본 좌표: 템플릿 :610-660·:699-709·CSS :1085-1243·미디어 .layout-header부.
// 태그 도메인 상태·핸들러는 useLayoutTags(뷰 1회 호출) → page로 수신한다.
import { useRoute } from 'vue-router'
import { ArrowDown, Expand, Fold, Search } from '@element-plus/icons-vue'
import { t } from '../../utils/i18n-runtime'

defineProps({
  page: {
    type: Object,
    required: true
  }
})

const route = useRoute()
</script>

<template>
      <el-header class="layout-header">
        <div class="header-top">
          <div class="header-left">
            <el-button class="collapse-button" circle @click="page.toggleCollapsed">
              <el-icon><component :is="page.collapsed ? Expand : Fold" /></el-icon>
            </el-button>

            <el-breadcrumb separator="/">
              <el-breadcrumb-item v-for="item in page.breadcrumbs" :key="item.path">
                {{ item.title }}
              </el-breadcrumb-item>
            </el-breadcrumb>
          </div>

          <div class="header-right">
            <el-button class="command-trigger" @click="page.openCommandPalette">
              <el-icon><Search /></el-icon>
              <span>{{ t('globalSearch') }}</span>
              <kbd>⌘K</kbd>
            </el-button>

            <el-dropdown>
              <div class="user-box">
                <div class="user-avatar">{{ (page.user.nickname || page.user.username || 'A').slice(0, 1).toUpperCase() }}</div>
                <span>{{ page.user.nickname || page.user.username || 'admin' }}</span>
                <el-icon><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="page.openProfile">{{ t('profile') }}</el-dropdown-item>
                  <el-dropdown-item divided @click="page.handleLogout">{{ t('logout') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>

        <div class="tabs-bar">
          <div
            v-for="tag in page.tagViews"
            :key="tag.path"
            class="tab-chip"
            :class="{ active: tag.path === route.path }"
            @click="page.handleTagClick(tag)"
            @contextmenu="page.openTagContextMenu($event, tag)"
          >
            <span>{{ tag.title }}</span>
            <button v-if="!page.isPinnedTag(tag)" type="button" class="tab-close" @click.stop="page.closeTag(tag)">×</button>
          </div>
        </div>
      </el-header>

    <div v-if="page.tagContextMenu.visible" class="tag-menu-mask" @click="page.closeTagContextMenu">
      <div
        class="tag-context-menu"
        :style="{ left: `${page.tagContextMenu.x}px`, top: `${page.tagContextMenu.y}px` }"
        @click.stop
      >
        <button type="button" @click="page.closeLeftTags">{{ t('closeLeftTabs') }}</button>
        <button type="button" @click="page.closeRightTags">{{ t('closeRightTabs') }}</button>
        <button type="button" @click="page.closeOtherTags">{{ t('closeOtherTabs') }}</button>
      </div>
    </div>
</template>

<style scoped>
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

.command-trigger { height: 34px; border-color: #d8e1f0; background: #fff; color: #64748b; }
.command-trigger:hover { border-color: #b9c8e9; background: #f3f6fd; color: #334155; }
.command-trigger kbd, .command-palette-head kbd, .command-result em { margin-left: 6px; padding: 1px 5px; border: 1px solid #d8e1f0; border-radius: 4px; color: #8391a7; font-family: inherit; font-size: 10px; font-style: normal; }

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


@media (max-width: 960px) {
  .layout-header {
    padding: 12px;
  }
}
</style>
