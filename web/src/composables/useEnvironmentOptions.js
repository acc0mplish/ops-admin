import { onMounted, ref } from 'vue'
import { queryOpsEnvironmentList } from '../api/ops'

export function useEnvironmentOptions() {
  const environmentOptions = ref([])
  const environmentLoading = ref(false)

  async function loadEnvironmentOptions() {
    environmentLoading.value = true
    try {
      environmentOptions.value = (await queryOpsEnvironmentList({ status: 1 })) || []
    } finally {
      environmentLoading.value = false
    }
  }

  function environmentName(code) {
    if (!code) return '未分配'
    const item = environmentOptions.value.find((option) => option.code === code)
    return item ? item.name : code
  }

  onMounted(loadEnvironmentOptions)

  return {
    environmentOptions,
    environmentLoading,
    environmentName,
    loadEnvironmentOptions
  }
}
