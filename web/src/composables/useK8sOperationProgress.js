import { computed, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { approveInfraTask, getInfraTask } from '../api/infra'
import {
  clearK8sOperationIdempotencyKey,
  createK8sOperationTask,
  K8S_OPERATION_TASK_TERMINAL_STATUSES,
  resolveK8sClusterConnectionUid,
  resolveK8sResourceUid
} from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'
import { getPermissions } from '../utils/auth'

// V2 §16.2 operation flow progress machinery — plan §3.2 H1, the mechanical
// extension of the E2 restart pattern (plan §3.5). One dialog serves every
// k8s mutation: N targets decompose into per-row §16.2 tasks, each submitted
// as plan → execute(Idempotency-Key) → approve (auto when permitted), then
// polled every 2s until every row reaches a terminal state or the 5-minute
// budget expires (rows flip to the timeout pseudo-status with the /infra/tasks
// exploration link). Poll/finish/prompt phrasing reuses the E2 restart keys.
const POLL_INTERVAL_MS = 2000
const POLL_BUDGET_MS = 5 * 60 * 1000

export function useK8sOperationProgress({ onFinish } = {}) {
  const visible = ref(false)
  const title = ref('')
  const rows = ref([])
  const finishHooks = []
  let pollTimer = null
  let pollStartedAt = 0
  let runDone = null
  let finishing = false

  const percent = computed(() => {
    const list = rows.value
    if (!list.length) return 0
    const done = list.filter((row) => K8S_OPERATION_TASK_TERMINAL_STATUSES.includes(row.status)).length
    return Math.round((done / list.length) * 100)
  })

  function statusText(status) {
    const keys = {
      pending: 'restartTaskPending',
      awaiting_approval: 'restartTaskAwaitingApproval',
      running: 'restartTaskRunning',
      succeeded: 'restartTaskSucceeded',
      failed: 'restartTaskFailed',
      timed_out: 'restartTaskFailed',
      cancelled: 'restartTaskCancelled',
      timeout: 'restartTaskTimeout'
    }
    return kt(keys[status] || 'restartTaskPending')
  }

  function statusTagType(status) {
    if (status === 'succeeded') return 'success'
    if (status === 'failed' || status === 'timed_out' || status === 'timeout') return 'danger'
    if (status === 'running') return 'warning'
    return 'info'
  }

  function stopPolling() {
    if (pollTimer) {
      window.clearInterval(pollTimer)
      pollTimer = null
    }
  }

  function startPolling() {
    stopPolling()
    pollTimer = window.setInterval(async () => {
      const active = rows.value.filter((row) => row.taskUid && !K8S_OPERATION_TASK_TERMINAL_STATUSES.includes(row.status))
      if (!active.length) {
        finish()
        return
      }
      if (Date.now() - pollStartedAt > POLL_BUDGET_MS) {
        active.forEach((row) => { row.status = 'timeout' })
        finish()
        return
      }
      for (const row of active) {
        try {
          const task = await getInfraTask(row.taskUid)
          if (task?.status) {
            const wasActive = !K8S_OPERATION_TASK_TERMINAL_STATUSES.includes(row.status)
            row.status = task.status
            // 종단 전환 시점에 소진된 키를 폐기한다 — 이후 재발화는 새 키 → 새 태스크.
            // (서버는 상태 무관 리플레이를 반환하므로 남은 키는 가짜 성공을 낳는다)
            if (wasActive && K8S_OPERATION_TASK_TERMINAL_STATUSES.includes(task.status)) {
              clearK8sOperationIdempotencyKey(row.resourceUid, row.op)
            }
          }
        } catch {
          // 일시적 조회 실패는 다음 폴링에서 재시도한다
        }
      }
      if (rows.value.every((row) => K8S_OPERATION_TASK_TERMINAL_STATUSES.includes(row.status))) {
        finish()
      }
    }, POLL_INTERVAL_MS)
  }

  async function finish() {
    if (finishing) return
    finishing = true
    stopPolling()
    const list = rows.value
    const failed = list.filter((row) => row.status === 'failed' || row.status === 'timed_out' || row.status === 'timeout')
    if (!failed.length) {
      ElMessage.success(kt('restartTaskAllSucceeded', { count: list.length }))
    } else {
      ElMessage.warning(kt('restartTaskPartialFailure', { count: failed.length, names: failed.map((row) => row.display).join(', ') }))
    }
    // 항상 함께 오는 갱신(onFinish) 먼저, 그다음 실행별로 밀린 후크.
    if (onFinish) {
      await onFinish()
    }
    while (finishHooks.length) {
      finishHooks.pop()()
    }
    // await run() 호출부 해제 — 전 행이 succeeded일 때만 참(거짓 성공 토스트 방지).
    if (runDone) {
      const resolve = runDone
      runDone = null
      resolve(list.length > 0 && list.every((row) => row.status === 'succeeded'))
    }
    finishing = false
  }

  // One submit per target row: resolve the V2 resource uid → plan → execute →
  // auto-approve when this user holds the opdef permission, else surface the
  // awaiting-external notice. Connection-uid resolution failure marks every
  // still-pending row failed (E2 precedent).
  async function run(clusterId, titleText, targets) {
    rows.value = targets.map((item) => ({
      key: item.display,
      display: item.display,
      op: item.op,
      target: item.target,
      payload: item.payload || {},
      resourceUid: '',
      taskUid: '',
      status: 'pending',
      notice: ''
    }))
    title.value = titleText
    visible.value = true
    stopPolling()
    runDone = null
    finishing = false
    pollStartedAt = Date.now()
    const permissions = getPermissions()
    try {
      const connectionUid = await resolveK8sClusterConnectionUid(clusterId)
      for (const row of rows.value) {
        try {
          const resourceUid = await resolveK8sResourceUid(connectionUid, row.target)
          row.resourceUid = resourceUid
          const { taskUid, permission } = await createK8sOperationTask(resourceUid, row.op, row.payload)
          row.taskUid = taskUid
          row.status = 'awaiting_approval'
          if (!permission || permissions.includes(permission)) {
            try {
              await approveInfraTask(taskUid)
              row.status = 'running'
            } catch {
              row.notice = 'restartTaskApprovalFailed'
            }
          } else {
            row.notice = 'restartTaskAwaitingExternal'
          }
        } catch {
          row.status = 'failed'
        }
      }
    } catch {
      rows.value.forEach((row) => {
        if (row.status === 'pending') row.status = 'failed'
      })
    }
    startPolling()
    // Resolves with "every row succeeded" when finish() runs (terminal states,
    // poll budget exhausted, or the dialog closed early).
    return new Promise((resolve) => {
      runDone = resolve
    })
  }

  // Dialog close during polling — settle the run with the current row states
  // so an awaiting caller is never left hanging.
  function closeOpProgress() {
    visible.value = false
    finish()
  }

  function pushFinishHook(hook) {
    finishHooks.push(hook)
  }

  onBeforeUnmount(stopPolling)
  return { visible, title, rows, percent, statusText, statusTagType, stopPolling, run, closeOpProgress, pushFinishHook }
}
