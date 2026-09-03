<script setup>
import { currentLocale, setLocale, t } from '../utils/i18n-runtime'

function switchLocale(value) {
  setLocale(value)
  window.location.reload()
}
</script>

<template>
  <div class="language-switch" :aria-label="t('switchLanguage')">
    <button
      type="button"
      :aria-label="t('language')"
      :class="{ active: currentLocale === 'ko-KR' }"
      @click="switchLocale('ko-KR')"
    >
      KO
    </button>
    <button
      type="button"
      :aria-label="t('english')"
      :class="{ active: currentLocale === 'en-US' }"
      @click="switchLocale('en-US')"
    >
      EN
    </button>
  </div>
</template>

<style scoped>
.language-switch {
  position: fixed;
  top: 14px;
  right: 14px;
  z-index: 5000;
  display: inline-flex;
  gap: 2px;
  padding: 3px;
  border: 1px solid rgba(148, 163, 184, 0.45);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 6px 20px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(8px);
}
.language-switch button {
  min-width: 34px;
  height: 28px;
  padding: 0 8px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  font-size: 11px;
  font-weight: 700;
}
.language-switch button.active {
  background: #172b4d;
  color: #fff;
}
</style>
