<script setup>
// I5-J (i5-plan §3.8·CW-4) — Host.vue 1,070행 분할 자식(호스트 폼 다이얼로그).
// 원본 좌표: 템플릿 :689-758·resetForm :93-109·openCreate :187-192·openEdit
// :213-234·openCopy :236-257·submit :259-280·CSS .ssh-line/.credential-line
// :986-1008(.credential-line은 HostImportDialogs 자식과 공유 — 복사 1건).
// `page` 주입(§5 #11 승계) — reactive 래핑으로 ref/reactive가 언랩된다.
import { ElMessage } from 'element-plus'
import { addAssetHost, assetHostInfo, updateAssetHost } from '../../../api/asset'
import { at } from '../../../utils/asset-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

// 원본 :93-109
function resetForm() {
  Object.assign(props.page.form, {
    id: undefined,
    hostName: '',
    groupId: undefined,
    groupIds: [],
    sshUser: '',
    sshIp: '',
    sshPort: 22,
    credentialId: undefined,
    connectionMode: 'direct',
    gatewayId: undefined,
    environment: '',
    status: 1,
    description: ''
  })
}

// 원본 :187-192
function openCreate() {
  props.page.isEdit = false
  props.page.isCopy = false
  resetForm()
  props.page.dialogVisible = true
}

// 원본 :213-234
async function openEdit(row) {
  props.page.isEdit = true
  props.page.isCopy = false
  const data = await assetHostInfo(row.id)
  resetForm()
  Object.assign(props.page.form, {
    id: data.id,
    hostName: data.hostName,
    groupId: data.groupId,
    groupIds: (data.hostGroups || []).map((item) => item.id).length ? (data.hostGroups || []).map((item) => item.id) : (data.groupId ? [data.groupId] : []),
    sshUser: data.sshUser,
    sshIp: data.sshIp,
    sshPort: data.sshPort || 22,
    credentialId: data.credentialId,
    connectionMode: data.connectionMode || 'direct',
    gatewayId: data.gatewayId || undefined,
    environment: data.environment || '',
    status: data.status || 1,
    description: data.description
  })
  props.page.dialogVisible = true
}

// 원본 :236-257
async function openCopy(row) {
  props.page.isEdit = false
  props.page.isCopy = true
  const data = await assetHostInfo(row.id)
  resetForm()
  Object.assign(props.page.form, {
    id: undefined,
    hostName: `${data.hostName || row.hostName || ''}-사본`,
    groupId: data.groupId,
    groupIds: (data.hostGroups || []).map((item) => item.id).length ? (data.hostGroups || []).map((item) => item.id) : (data.groupId ? [data.groupId] : []),
    sshUser: data.sshUser,
    sshIp: data.sshIp,
    sshPort: data.sshPort || 22,
    credentialId: data.credentialId,
    connectionMode: data.connectionMode || 'direct',
    gatewayId: data.gatewayId || undefined,
    environment: data.environment || '',
    status: data.status || 1,
    description: data.description
  })
  props.page.dialogVisible = true
}

// 원본 :259-280
async function submit() {
  const form = props.page.form
  if (!form.hostName || !form.groupIds.length || !form.environment || !form.sshUser || !form.sshIp || !form.credentialId) {
    ElMessage.warning(at('enterHostRequiredFields'))
    return
  }
  if (form.connectionMode === 'gateway' && !form.gatewayId) {
    ElMessage.warning(at('selectGatewayWarning'))
    return
  }
  form.groupId = form.groupIds[0]

  if (props.page.isEdit) {
    await updateAssetHost(form)
    ElMessage.success(at('hostUpdated'))
  } else {
    await addAssetHost(form)
    ElMessage.success(props.page.isCopy ? at('hostCopied') : at('hostCreated'))
  }
  props.page.isCopy = false
  props.page.dialogVisible = false
  await props.page.loadData()
}

defineExpose({
  openCreate,
  openEdit,
  openCopy
})
</script>

<template>
  <el-dialog v-model="page.dialogVisible" :title="page.isEdit ? at('hostEditTitle') : (page.isCopy ? at('hostCloneTitle') : at('hostAddTitle'))" width="640px">
    <el-form label-width="96px">
      <el-row :gutter="18">
        <el-col :span="12">
          <el-form-item :label="at('hostNameLabel')" required>
            <el-input v-model="page.form.hostName" :placeholder="at('enterHostName')" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="at('hostGroupLabel')" required>
            <el-select v-model="page.form.groupIds" multiple collapse-tags collapse-tags-tooltip filterable :placeholder="at('selectGroup')" style="width: 100%">
              <el-option v-for="item in page.groupOptions" :key="item.id" :value="item.id" :label="item.name" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="at('sshConnectionLabel')" required>
            <div class="ssh-line">
              <el-input v-model="page.form.sshUser" :placeholder="at('usernamePlaceholder')" />
              <span>@</span>
              <el-input v-model="page.form.sshIp" :placeholder="at('hostAddressPlaceholder')" />
              <span>-p</span>
              <el-input-number v-model="page.form.sshPort" :min="1" :max="65535" controls-position="right" />
            </div>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="at('authCredentialLabel')" required>
            <div class="credential-line">
              <el-select v-model="page.form.credentialId" clearable filterable :placeholder="at('selectCredential')">
                <el-option v-for="item in page.credentialOptions" :key="item.id" :value="item.id" :label="item.name" />
              </el-select>
              <el-button color="#f59e0b" @click="page.goCredential">{{ at('createCredential') }}</el-button>
            </div>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="at('environmentLabel')" required>
            <el-select v-model="page.form.environment" :loading="page.environmentLoading" :placeholder="at('selectEnvironment')" style="width: 100%">
              <el-option v-for="item in page.environmentOptions" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="at('connectionModeLabel')">
            <el-radio-group v-model="page.form.connectionMode">
              <el-radio-button label="direct">{{ at('directConnection') }}</el-radio-button>
              <el-radio-button label="gateway">{{ at('viaGateway') }}</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <el-col v-if="page.form.connectionMode === 'gateway'" :span="12">
          <el-form-item :label="at('accessGatewayLabel')" required>
            <el-select v-model="page.form.gatewayId" filterable :placeholder="at('selectGateway')" style="width: 100%">
              <el-option v-for="item in page.gatewayOptions" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label="at('noteLabel')">
            <el-input v-model="page.form.description" type="textarea" :rows="3" :placeholder="at('enterNote')" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <template #footer>
      <el-button @click="page.dialogVisible = false">{{ at('cancel') }}</el-button>
      <el-button type="primary" @click="submit">{{ at('confirm') }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 원본 :986-1008 — .credential-line은 HostImportDialogs 자식과 공유(복사 1건) */
.ssh-line,
.credential-line {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.ssh-line .el-input:first-child {
  width: 130px;
}

.ssh-line .el-input:nth-child(3) {
  flex: 1;
}

.ssh-line .el-input-number {
  width: 96px;
}

.credential-line .el-select {
  flex: 1;
}
</style>
