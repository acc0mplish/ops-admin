<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import BasicConfig from './BasicConfig.vue'
import { getLDAPConfig, queryDeptList, querySysPostVoList, querySysRoleVoList, saveLDAPConfig, testLDAPConfig } from '../../api/system'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const roles = ref([])
const depts = ref([])
const posts = ref([])
const passwordConfigured = ref(false)

const form = reactive({
  enabled: false,
  serverUrl: '',
  tlsMode: 'starttls',
  insecureSkipVerify: false,
  bindDn: '',
  bindPassword: '',
  baseDn: '',
  userFilter: '(&(objectClass=person)(uid={{username}}))',
  usernameAttribute: 'uid',
  displayAttribute: 'displayName',
  emailAttribute: 'mail',
  phoneAttribute: 'mobile',
  defaultRoleId: undefined,
  defaultDeptId: undefined,
  defaultPostId: undefined
})

async function loadData() {
  loading.value = true
  try {
    const [config, roleData, deptData, postData] = await Promise.all([
      getLDAPConfig(), querySysRoleVoList(), queryDeptList(), querySysPostVoList()
    ])
    Object.assign(form, config || {}, { bindPassword: '' })
    passwordConfigured.value = Boolean(config?.bindPasswordSet)
    roles.value = roleData || []
    depts.value = deptData || []
    posts.value = postData || []
  } finally {
    loading.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await testLDAPConfig(form)
    ElMessage.success('LDAP 连接与绑定验证成功')
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const data = await saveLDAPConfig(form)
    Object.assign(form, data || {}, { bindPassword: '' })
    passwordConfigured.value = Boolean(data?.bindPasswordSet)
    ElMessage.success('LDAP 配置已保存')
  } finally {
    saving.value = false
  }
}

onMounted(loadData)
</script>

<template>
  <div class="settings-page" v-loading="loading">
    <div class="settings-head">
      <div>
        <h2>系统设置</h2>
        <p>维护平台外观与统一认证集成。LDAP 密码仅保存在服务端，页面不会回显。</p>
      </div>
    </div>

    <el-tabs class="settings-tabs">
      <el-tab-pane label="通用设置">
        <BasicConfig />
      </el-tab-pane>
      <el-tab-pane label="LDAP 认证集成">
        <section class="ldap-card">
          <div class="section-head">
            <div>
              <h3>LDAP / Active Directory</h3>
              <p>配置目录服务并将 LDAP 用户同步到本地用户管理。</p>
            </div>
            <el-switch v-model="form.enabled" active-text="启用 LDAP" inactive-text="未启用" />
          </div>

          <el-alert type="info" :closable="false" show-icon>
            <template #title>用户过滤器支持 <code v-pre>{{username}}</code> 占位符；同步时会以 LDAP 中的用户名创建或更新本地用户资料。</template>
          </el-alert>

          <el-form label-width="130px" class="ldap-form">
            <div class="form-grid">
              <el-form-item label="LDAP 服务地址" required>
                <el-input v-model="form.serverUrl" placeholder="ldap://ldap.example.com:389 或 ldaps://ldap.example.com:636" />
              </el-form-item>
              <el-form-item label="传输加密">
                <el-radio-group v-model="form.tlsMode">
                  <el-radio value="starttls">StartTLS</el-radio>
                  <el-radio value="ldaps">LDAPS</el-radio>
                  <el-radio value="plain">明文 LDAP</el-radio>
                </el-radio-group>
              </el-form-item>
              <el-form-item label="绑定 DN">
                <el-input v-model="form.bindDn" placeholder="cn=ops-admin,ou=service,dc=example,dc=com" />
              </el-form-item>
              <el-form-item label="绑定密码">
                <el-input v-model="form.bindPassword" type="password" show-password :placeholder="passwordConfigured ? '已配置，留空则不修改' : '请输入绑定密码'" />
              </el-form-item>
              <el-form-item label="Base DN" required>
                <el-input v-model="form.baseDn" placeholder="ou=people,dc=example,dc=com" />
              </el-form-item>
              <el-form-item label="用户过滤器">
                <el-input v-model="form.userFilter" placeholder="(&(objectClass=person)(uid={{username}}))" />
              </el-form-item>
            </div>

            <el-divider content-position="left">LDAP 用户字段映射</el-divider>
            <div class="form-grid">
              <el-form-item label="用户名属性" required><el-input v-model="form.usernameAttribute" placeholder="uid" /></el-form-item>
              <el-form-item label="显示名属性"><el-input v-model="form.displayAttribute" placeholder="displayName" /></el-form-item>
              <el-form-item label="邮箱属性"><el-input v-model="form.emailAttribute" placeholder="mail" /></el-form-item>
              <el-form-item label="手机号属性"><el-input v-model="form.phoneAttribute" placeholder="mobile" /></el-form-item>
            </div>

            <el-divider content-position="left">同步默认归属</el-divider>
            <div class="form-grid">
              <el-form-item label="默认角色" required>
                <el-select v-model="form.defaultRoleId" clearable placeholder="新增 LDAP 用户时必选" style="width:100%">
                  <el-option v-for="item in roles" :key="item.id" :label="item.roleName" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="默认部门">
                <el-select v-model="form.defaultDeptId" clearable placeholder="可选" style="width:100%">
                  <el-option v-for="item in depts" :key="item.id" :label="item.deptName" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="默认岗位">
                <el-select v-model="form.defaultPostId" clearable placeholder="可选" style="width:100%">
                  <el-option v-for="item in posts" :key="item.id" :label="item.postName" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="证书校验">
                <el-switch v-model="form.insecureSkipVerify" active-text="跳过校验" inactive-text="校验证书" />
              </el-form-item>
            </div>
          </el-form>

          <div class="action-bar">
            <el-button :loading="testing" @click="handleTest">测试连接</el-button>
            <el-button type="primary" :loading="saving" @click="handleSave">保存 LDAP 配置</el-button>
          </div>
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
