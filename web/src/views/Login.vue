<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getPublicSystemConfig, login } from '../api/system'
import { setMenus, setPermissions, setToken, setUser } from '../utils/auth'
import { applySystemTheme, getSystemConfig, resolveSystemAsset, setSystemConfig } from '../utils/system-config'
import { t } from '../utils/i18n-runtime'

const router = useRouter()
const loading = ref(false)
const config = ref(getSystemConfig())
const form = reactive({
  username: '',
  password: ''
})

const logoText = computed(() => {
  if (config.value.logoType === 'text' && config.value.logoValue) {
    return String(config.value.logoValue).slice(0, 2).toUpperCase()
  }
  return 'OA'
})

const logoImage = computed(() => {
  if (config.value.logoType === 'upload' || config.value.logoType === 'url') {
    return resolveSystemAsset(config.value.logoValue)
  }
  return ''
})

const loginBackgroundStyle = computed(() => {
  const bg = resolveSystemAsset(config.value.loginBackground)
  if (config.value.useLoginBackground && bg) {
    return {
      backgroundImage: `linear-gradient(135deg, rgba(18, 27, 71, 0.82), rgba(41, 64, 149, 0.72)), url(${bg})`
    }
  }
  return {}
})

async function loadSystemConfig() {
  try {
    const data = await getPublicSystemConfig()
    config.value = data || {}
    setSystemConfig(config.value)
    applySystemTheme(config.value)
  } catch {
    applySystemTheme(config.value)
  }
}

async function submit() {
  loading.value = true
  try {
    const data = await login(form)
    setToken(data.token, data.accessTokenExpiresAt)
    setUser(data.sysAdmin)
    setMenus(data.leftMenuList || [])
    setPermissions(data.permissionList || [])
    if (data.systemConfig) {
      setSystemConfig(data.systemConfig)
      applySystemTheme(data.systemConfig)
    }
    ElMessage.success(t('saveSuccess'))
    router.push('/dashboard')
  } finally {
    loading.value = false
  }
}

onMounted(loadSystemConfig)
</script>

<template>
  <div class="login-shell">
    <div class="login-backdrop" :style="loginBackgroundStyle"></div>
    <div class="login-panel">
      <section class="login-hero">
        <div class="hero-badge">
          <img v-if="logoImage" :src="logoImage" alt="logo" class="hero-logo-image" />
          <template v-else>{{ logoText }}</template>
        </div>
        <h1>{{ config.loginTitle || config.siteName || 'Ops Admin' }}</h1>
        <p>{{ config.loginSubtitle || config.siteSlogan || t('loginHeroSubtitle') }}</p>
        <div class="hero-meta">
          <span>System</span>
          <span>RBAC</span>
          <span>Audit</span>
        </div>
      </section>

      <section class="login-form">
        <div class="form-head">
          <h2>{{ t('loginWelcome') }}</h2>
          <p>{{ t('loginHint') }}</p>
        </div>
        <el-form label-position="top" @submit.prevent>
          <el-form-item :label="t('username')">
            <el-input v-model="form.username" :placeholder="t('username')" />
          </el-form-item>
          <el-form-item :label="t('password')">
            <el-input v-model="form.password" type="password" show-password :placeholder="t('password')" />
          </el-form-item>
          <el-button type="primary" class="submit-btn" :loading="loading" @click="submit">{{ t('loginSystem') }}</el-button>
        </el-form>
      </section>
    </div>
  </div>
</template>
