<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
import { at } from '../../utils/asset-i18n'

const router = useRouter()
const loading = ref(true)

async function enterWorkbench() {
  try {
    const data = await queryAssetDatabaseList({ pageNum: 1, pageSize: 500, status: '1' })
    const database = (data.list || []).find((item) => item.connectStatus === 1)
    if (!database) {
      ElMessage.warning(at('noDatabaseConnection'))
      await router.replace('/assets/databases')
      return
    }
    await router.replace({ name: 'DatabaseWorkbench', params: { id: database.id } })
  } finally {
    loading.value = false
  }
}

onMounted(enterWorkbench)
</script>

<template>
  <div v-loading="loading" class="workbench-loading">
    <span>{{ at('enteringDatabaseWorkbench') }}</span>
  </div>
</template>

<style scoped>
.workbench-loading {
  display: grid;
  min-height: calc(100vh - 180px);
  place-items: center;
  color: var(--el-text-color-secondary);
}
</style>
