<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import {
  executeOpsCommand,
  executeOpsFileDispatch,
  executeOpsScript,
  queryOpsScriptOptions
} from '../../api/ops'

const activeTab = ref('command')
const submitting = ref(false)
const hostOptions = ref([])
const groupOptions = ref([])
const scriptOptions = ref([])
const latestResult = ref(null)

const commandForm = reactive({
  title: '',
  commandText: '',
  parameters: '',
  hostIds: [],
  groupIds: [],
  concurrency: 5
})

const scriptForm = reactive({
  title: '',
  scriptId: undefined,
  parameters: '',
  hostIds: [],
  groupIds: [],
  concurrency: 5
})

const fileForm = reactive({
  title: '',
  sourceType: 'upload',
  sourceHostId: undefined,
  sourcePath: '',
  targetPath: '',
  hostIds: [],
  groupIds: [],
  concurrency: 5,
  overwrite: false,
  file: null
})

const flatGroupOptions = computed(() => flattenGroups(groupOptions.value))

function flattenGroups(nodes = [], prefix = '') {
  return nodes.flatMap((item) => {
    const label = prefix ? `${prefix} / ${item.name}` : item.name
    return [{ label, value: item.id }, ...flattenGroups(item.children || [], label)]
  })
}

async function loadOptions() {
  const [hosts, groups, scripts] = await Promise.all([
    queryAssetHostList({ pageNum: 1, pageSize: 500 }),
    queryAssetHostGroupList(),
    queryOpsScriptOptions()
  ])
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
  scriptOptions.value = scripts || []
}

function handleFileChange(file) {
  fileForm.file = file?.raw || null
}

function validateTargets(hostIds, groupIds) {
  if (!hostIds.length && !groupIds.length) {
    ElMessage.warning('Host 또는 Host Group을 하나 이상 선택하십시오.')
    return false
  }
  return true
}

async function submitCommand() {
  if (!commandForm.commandText.trim()) {
    ElMessage.warning('실행 Command를 입력하십시오.')
    return
  }
  if (!validateTargets(commandForm.hostIds, commandForm.groupIds)) return
  submitting.value = true
  try {
    latestResult.value = await executeOpsCommand(commandForm)
    ElMessage.success('Command 실행을 완료하고 결과를 Execution History에 기록했습니다.')
  } finally {
    submitting.value = false
  }
}

async function submitScript() {
  if (!scriptForm.scriptId) {
    ElMessage.warning('Script를 선택하십시오.')
    return
  }
  if (!validateTargets(scriptForm.hostIds, scriptForm.groupIds)) return
  submitting.value = true
  try {
    latestResult.value = await executeOpsScript(scriptForm)
    ElMessage.success('Script 실행을 완료하고 결과를 Execution History에 기록했습니다.')
  } finally {
    submitting.value = false
  }
}

async function submitFileDispatch() {
  if (!fileForm.targetPath.trim()) {
    ElMessage.warning('Target Path를 입력하십시오.')
    return
  }
  if (!validateTargets(fileForm.hostIds, fileForm.groupIds)) return
  if (fileForm.sourceType === 'upload' && !fileForm.file) {
    ElMessage.warning('배포할 File을 Upload하십시오.')
    return
  }
  if (fileForm.sourceType === 'server' && (!fileForm.sourceHostId || !fileForm.sourcePath.trim())) {
    ElMessage.warning('Source Server를 선택하고 Source File Path를 입력하십시오.')
    return
  }

  const formData = new FormData()
  formData.append('title', fileForm.title)
  formData.append('sourceType', fileForm.sourceType)
  formData.append('sourceHostId', String(fileForm.sourceHostId || 0))
  formData.append('sourcePath', fileForm.sourcePath)
  formData.append('targetPath', fileForm.targetPath)
  formData.append('hostIds', JSON.stringify(fileForm.hostIds))
  formData.append('groupIds', JSON.stringify(fileForm.groupIds))
  formData.append('concurrency', String(fileForm.concurrency))
  formData.append('overwrite', String(fileForm.overwrite))
  if (fileForm.file) {
    formData.append('file', fileForm.file)
  }

  submitting.value = true
  try {
    latestResult.value = await executeOpsFileDispatch(formData)
    ElMessage.success('File Distribution을 완료하고 결과를 Execution History에 기록했습니다.')
  } finally {
    submitting.value = false
  }
}

onMounted(loadOptions)
</script>

<template>
  <div class="ops-page">
    <div class="page-card">
      <div class="page-header">
        <div>
          <h2 class="page-title">Quick Execution</h2>
          <p class="page-desc">SSH로 Command, Script를 실행하거나 여러 Host에 File을 배포합니다. Concurrency 기본값은 5, 최대 10입니다.</p>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="ops-tabs">
        <el-tab-pane label="Command 실행" name="command">
          <div class="form-grid">
            <el-form label-width="100px">
              <el-form-item label="Task 이름">
                <el-input v-model="commandForm.title" placeholder="선택 사항. 비워두면 자동 생성합니다." />
              </el-form-item>
              <el-form-item label="실행 Command" required>
                <el-input v-model="commandForm.commandText" type="textarea" :rows="8" placeholder="예: systemctl status nginx" />
              </el-form-item>
              <el-form-item label="실행 Parameter">
                <el-input v-model="commandForm.parameters" placeholder="예: --env prod" />
              </el-form-item>
              <el-form-item label="Target Host">
                <el-select v-model="commandForm.hostIds" multiple filterable collapse-tags style="width: 100%" placeholder="Host 선택">
                  <el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="Host Group">
                <el-select v-model="commandForm.groupIds" multiple filterable collapse-tags style="width: 100%" placeholder="Host Group 선택">
                  <el-option v-for="item in flatGroupOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="Concurrency">
                <el-input-number v-model="commandForm.concurrency" :min="1" :max="10" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="submitting" @click="submitCommand">즉시 실행</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>

        <el-tab-pane label="Script 실행" name="script">
          <el-form label-width="100px">
            <el-form-item label="Task 이름">
              <el-input v-model="scriptForm.title" placeholder="선택 사항. 비워두면 자동 생성합니다." />
            </el-form-item>
            <el-form-item label="Script" required>
              <el-select v-model="scriptForm.scriptId" filterable style="width: 100%" placeholder="Script Library에서 Script 선택">
                <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="실행 Parameter">
              <el-input v-model="scriptForm.parameters" placeholder="비워두면 Script 기본 Parameter를 사용합니다." />
            </el-form-item>
            <el-form-item label="Target Host">
              <el-select v-model="scriptForm.hostIds" multiple filterable collapse-tags style="width: 100%">
                <el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="Host Group">
              <el-select v-model="scriptForm.groupIds" multiple filterable collapse-tags style="width: 100%">
                <el-option v-for="item in flatGroupOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="Concurrency">
              <el-input-number v-model="scriptForm.concurrency" :min="1" :max="10" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="submitting" @click="submitScript">즉시 실행</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="File Distribution" name="file">
          <el-form label-width="110px">
            <el-form-item label="Task 이름">
              <el-input v-model="fileForm.title" placeholder="선택 사항. 비워두면 자동 생성합니다." />
            </el-form-item>
            <el-form-item label="Source 방식">
              <el-radio-group v-model="fileForm.sourceType">
                <el-radio value="upload">로컬 Upload</el-radio>
                <el-radio value="server">Server Source File</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="fileForm.sourceType === 'upload'" label="File Upload">
              <el-upload :auto-upload="false" :show-file-list="true" :limit="1" :on-change="handleFileChange">
                <el-button>File 선택</el-button>
              </el-upload>
            </el-form-item>
            <template v-else>
              <el-form-item label="Source Server">
                <el-select v-model="fileForm.sourceHostId" filterable style="width: 100%" placeholder="Source Server 선택">
                  <el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="Source File Path">
                <el-input v-model="fileForm.sourcePath" placeholder="예: /data/release/app.tar.gz" />
              </el-form-item>
            </template>
            <el-form-item label="Target Path" required>
              <el-input v-model="fileForm.targetPath" placeholder="예: /opt/apps/app.tar.gz" />
            </el-form-item>
            <el-form-item label="Target Host">
              <el-select v-model="fileForm.hostIds" multiple filterable collapse-tags style="width: 100%">
                <el-option v-for="item in hostOptions" :key="item.id" :label="`${item.hostName} (${item.sshIp || item.privateIp || '-'})`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="Host Group">
              <el-select v-model="fileForm.groupIds" multiple filterable collapse-tags style="width: 100%">
                <el-option v-for="item in flatGroupOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="Concurrency">
              <el-input-number v-model="fileForm.concurrency" :min="1" :max="10" />
            </el-form-item>
            <el-form-item label="기존 File 덮어쓰기">
              <el-switch v-model="fileForm.overwrite" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="submitting" @click="submitFileDispatch">즉시 배포</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>

    <div v-if="latestResult?.task" class="page-card">
      <div class="result-header">
        <div>
          <h3>최근 실행 결과</h3>
          <p>{{ latestResult.task.title }} · {{ latestResult.task.summary || '-' }}</p>
        </div>
        <el-tag :type="latestResult.task.status === 'success' ? 'success' : latestResult.task.status === 'partial' ? 'warning' : 'danger'">
          {{ latestResult.task.status }}
        </el-tag>
      </div>
      <el-table :data="latestResult.results || []" border>
        <el-table-column prop="hostName" label="Host" min-width="180" />
        <el-table-column prop="sshIp" label="SSH IP" min-width="140" />
        <el-table-column prop="status" label="상태" width="100" />
        <el-table-column prop="exitCode" label="Exit Code" width="90" />
        <el-table-column prop="durationMs" label="소요 시간(ms)" width="110" />
        <el-table-column prop="stdout" label="Output" min-width="260" show-overflow-tooltip />
        <el-table-column prop="errorText" label="오류" min-width="220" show-overflow-tooltip />
      </el-table>
    </div>
  </div>
</template>

<style scoped>
.ops-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.page-title {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 700;
  color: #14213d;
}

.page-desc {
  margin: 0;
  color: #7282a0;
}

.ops-tabs :deep(.el-tabs__header) {
  margin-bottom: 18px;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.result-header h3 {
  margin: 0 0 6px;
}

.result-header p {
  margin: 0;
  color: #7282a0;
}
</style>
