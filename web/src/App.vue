<script setup>
import { computed } from 'vue'
import koLocale from 'element-plus/es/locale/lang/ko'
import enLocale from 'element-plus/es/locale/lang/en'
import LanguageSwitch from './components/LanguageSwitch.vue'
import { currentLocale } from './utils/i18n-runtime'

const epLocale = computed(() => (currentLocale.value === 'ko-KR' ? koLocale : enLocale))
</script>

<template>
  <el-config-provider :locale="epLocale">
    <router-view />
    <LanguageSwitch />
  </el-config-provider>
</template>
