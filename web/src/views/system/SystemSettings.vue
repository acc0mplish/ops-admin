<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import BasicConfig from './BasicConfig.vue'
import { getLDAPConfig, queryDeptList, querySysPostVoList, querySysRoleVoList, saveLDAPConfig, testLDAPConfig } from '../../api/system'
import { translateEntity } from '../../utils/i18n-runtime'
import { lt } from '../../utils/ldap-i18n'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const roles = ref([])
const depts = ref([])
const posts = ref([])
const passwordConfigured = ref(false)
const form = reactive({ enabled: false, serverUrl: '', tlsMode: 'starttls', insecureSkipVerify: false, bindDn: '', bindPassword: '', baseDn: '', userFilter: '(&(objectClass=person)(uid={{username}}))', usernameAttribute: 'uid', displayAttribute: 'displayName', emailAttribute: 'mail', phoneAttribute: 'mobile', defaultRoleId: undefined, defaultDeptId: undefined, defaultPostId: undefined })

async function loadData() {
  loading.value = true
  try {
    const [config, roleData, deptData, postData] = await Promise.all([getLDAPConfig(), querySysRoleVoList(), queryDeptList(), querySysPostVoList()])
    Object.assign(form, config || {}, { bindPassword: '' })
    passwordConfigured.value = Boolean(config?.bindPasswordSet)
    roles.value = roleData || []
    depts.value = deptData || []
    posts.value = postData || []
  } finally { loading.value = false }
}
async function handleTest() { testing.value = true; try { await testLDAPConfig(form); ElMessage.success(lt('connectionSuccess')) } finally { testing.value = false } }
async function handleSave() { saving.value = true; try { const data = await saveLDAPConfig(form); Object.assign(form, data || {}, { bindPassword: '' }); passwordConfigured.value = Boolean(data?.bindPasswordSet); ElMessage.success(lt('saved')) } finally { saving.value = false } }
onMounted(loadData)
</script>

<template>
  <div class="settings-page" v-loading="loading">
    <div class="settings-head"><div><h2>{{ lt('settings') }}</h2><p>{{ lt('settingsDesc') }}</p></div></div>
    <el-tabs class="settings-tabs">
      <el-tab-pane :label="lt('general')"><BasicConfig /></el-tab-pane>
      <el-tab-pane :label="lt('integration')">
        <section class="ldap-card">
          <div class="section-head"><div><h3>LDAP / Active Directory</h3><p>{{ lt('ldapDesc') }}</p></div><el-switch v-model="form.enabled" :active-text="lt('enabled')" :inactive-text="lt('disabled')" /></div>
          <el-alert type="info" :closable="false" show-icon><template #title>{{ lt('filterHint') }}</template></el-alert>
          <el-form label-width="130px" class="ldap-form">
            <div class="form-grid">
              <el-form-item :label="lt('serverUrl')" required><el-input v-model="form.serverUrl" placeholder="ldap://ldap.example.com:389 / ldaps://ldap.example.com:636" /></el-form-item>
              <el-form-item :label="lt('transport')"><el-radio-group v-model="form.tlsMode"><el-radio value="starttls">StartTLS</el-radio><el-radio value="ldaps">LDAPS</el-radio><el-radio value="plain">{{ lt('plain') }}</el-radio></el-radio-group></el-form-item>
              <el-form-item :label="lt('bindDn')"><el-input v-model="form.bindDn" placeholder="cn=ops-admin,ou=service,dc=example,dc=com" /></el-form-item>
              <el-form-item :label="lt('bindPassword')"><el-input v-model="form.bindPassword" type="password" show-password :placeholder="passwordConfigured ? lt('passwordConfigured') : lt('passwordRequired')" /></el-form-item>
              <el-form-item label="Base DN" required><el-input v-model="form.baseDn" placeholder="ou=people,dc=example,dc=com" /></el-form-item>
              <el-form-item :label="lt('userFilter')"><el-input v-model="form.userFilter" placeholder="(&(objectClass=person)(uid={{username}}))" /></el-form-item>
            </div>
            <el-divider content-position="left">{{ lt('fieldMapping') }}</el-divider>
            <div class="form-grid"><el-form-item :label="lt('usernameAttr')" required><el-input v-model="form.usernameAttribute" placeholder="uid" /></el-form-item><el-form-item :label="lt('displayAttr')"><el-input v-model="form.displayAttribute" placeholder="displayName" /></el-form-item><el-form-item :label="lt('emailAttr')"><el-input v-model="form.emailAttribute" placeholder="mail" /></el-form-item><el-form-item :label="lt('phoneAttr')"><el-input v-model="form.phoneAttribute" placeholder="mobile" /></el-form-item></div>
            <el-divider content-position="left">{{ lt('defaults') }}</el-divider>
            <div class="form-grid">
              <el-form-item :label="lt('defaultRole')" required><el-select v-model="form.defaultRoleId" clearable :placeholder="lt('roleRequired')" style="width:100%"><el-option v-for="item in roles" :key="item.id" :label="translateEntity(item.roleName, item.roleName)" :value="item.id" /></el-select></el-form-item>
              <el-form-item :label="lt('defaultDept')"><el-select v-model="form.defaultDeptId" clearable :placeholder="lt('optional')" style="width:100%"><el-option v-for="item in depts" :key="item.id" :label="translateEntity(item.deptName, item.deptName)" :value="item.id" /></el-select></el-form-item>
              <el-form-item :label="lt('defaultPost')"><el-select v-model="form.defaultPostId" clearable :placeholder="lt('optional')" style="width:100%"><el-option v-for="item in posts" :key="item.id" :label="translateEntity(item.postName, item.postName)" :value="item.id" /></el-select></el-form-item>
              <el-form-item :label="lt('certificateValidation')"><el-switch v-model="form.insecureSkipVerify" :active-text="lt('skipValidation')" :inactive-text="lt('validateCertificate')" /></el-form-item>
            </div>
          </el-form>
          <div class="action-bar"><el-button :loading="testing" @click="handleTest">{{ lt('testConnection') }}</el-button><el-button type="primary" :loading="saving" @click="handleSave">{{ lt('saveLdap') }}</el-button></div>
        </section>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.settings-page { min-height: 100%; }
.settings-head { margin-bottom: 16px; }
.settings-head h2 { margin: 0; font-size: 28px; color: #14213d; }
.settings-head p { margin: 8px 0 0; color: #75829a; }
.settings-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
.ldap-card { padding: 24px; border: 1px solid #e4ebf6; border-radius: 8px; background: #fff; }
.section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.section-head h3 { margin: 0; font-size: 18px; color: #1b2a4a; }
.section-head p { margin: 7px 0 0; color: #7c8aa5; }
.ldap-form { margin-top: 22px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 28px; }
.action-bar { display: flex; justify-content: flex-end; gap: 12px; padding-top: 18px; border-top: 1px solid #edf1f7; }
@media (max-width: 1000px) { .form-grid { grid-template-columns: 1fr; } }
</style>
