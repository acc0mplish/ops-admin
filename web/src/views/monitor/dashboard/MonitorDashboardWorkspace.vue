<script setup>
// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 자식.
// 원본 좌표는 각 블록 주석과 부모 커밋 메시지 참조. `page` 주입(§5 #11 승계):
// reactive(page)로 ref/computed가 언랩되어 v-model 재배선이 동작한다.
import { uiT } from '../../../utils/english-hardcoding-i18n'
import { mt } from '../../../utils/monitor-i18n'
defineProps({
  page: {
    type: Object,
    required: true
  }
})
</script>

<template>
    <section class="dashboard-workspace">
      <div class="workspace-title">
        <span class="brand-mark">M</span>
        <div>
          <strong>{{ uiT('monitoringDashboard') }}</strong>
          <p>{{ pageDescription }}</p>
        </div>
      </div>

      <div class="dashboard-switcher">
        <el-scrollbar>
          <div class="dashboard-list">
          <button
            v-for="item in page.visibleDashboards"
            :key="item.id"
            class="dashboard-item"
            :class="{ active: item.id === page.activeDashboardId }"
            @click="page.loadDashboard(item.id)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ mt('panelCountTotal', { count: item.panelCount || 0 }) }}</span>
          </button>
          <div v-if="!page.visibleDashboards.length" class="empty-switcher">{{ mt('titleEmpty', { title: page.pageTitle }) }}</div>
          </div>
        </el-scrollbar>
      </div>

      <el-button class="create-screen-btn" type="primary" @click="page.openCreateDashboard">{{ mt('createTitle', { title: page.pageTitle }) }}</el-button>
    </section>
</template>

<style scoped>
.dashboard-workspace {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 18px;
  background: #fff;
  border: 1px solid #dce7f7;
  box-shadow: 0 10px 24px rgba(36, 54, 90, 0.07);
}
.workspace-title {
  display: flex;
  align-items: center;
  gap: 12px;
}
.brand-mark {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: linear-gradient(135deg, #2563eb, #60a5fa);
  color: #fff;
  font-weight: 800;
}
.workspace-title strong,
.workspace-title p {
  display: block;
  margin: 0;
}
.workspace-title strong {
  color: #10213f;
  font-size: 16px;
}
.workspace-title p {
  margin-top: 4px;
  color: #7282a0;
  font-size: 13px;
}
.create-screen-btn {
  min-width: 132px;
  height: 42px;
}
.dashboard-switcher {
  min-width: 0;
  padding: 8px;
  border-radius: 14px;
  background: #f3f7fd;
  border: 1px solid #e2ebf7;
}
.dashboard-list {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 54px;
}
.dashboard-item {
  flex: 0 0 190px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: transparent;
  color: #41516e;
  text-align: left;
  cursor: pointer;
  transition: 0.18s ease;
}
.dashboard-item strong,
.dashboard-item span {
  display: block;
}
.dashboard-item span {
  margin-top: 5px;
  color: #8a99b4;
  font-size: 12px;
}
.dashboard-item.active,
.dashboard-item:hover {
  background: #fff;
  border-color: #d7e4f5;
  color: #17335f;
  box-shadow: 0 8px 18px rgba(47, 99, 191, 0.1);
}
.dashboard-item.active span,
.dashboard-item:hover span {
  color: #6b7b95;
}
.empty-switcher {
  padding: 0 14px;
  color: #8a99b4;
  white-space: nowrap;
}
.dashboard-workspace {
  padding: 12px 14px;
  border-color: #d8e0eb;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.04);
}
.dashboard-switcher {
  padding: 4px;
  border-color: #dce4ef;
  border-radius: 6px;
  background: #f4f7fb;
}
.dashboard-list {
  min-height: 46px;
  gap: 4px;
}
.dashboard-item {
  flex-basis: 176px;
  padding: 8px 10px;
  border-radius: 5px;
}
.dashboard-item.active,
.dashboard-item:hover {
  box-shadow: 0 1px 4px rgba(30, 64, 175, 0.08);
}
@media (max-width: 1280px) {
  .dashboard-workspace {
    grid-template-columns: 1fr;
    align-items: stretch;
  }
  .create-screen-btn {
    width: 100%;
  }
}
</style>
