// Slice A UI-flow E2E (plan N15 — the G2 lane of the J9 two-layer split).
// The Go trace (backend/e2e/slicea, G1) owns process control and crash
// injection; this lane proves the §25 mutation end to end through the real
// console: login → provider connections → K8s inventory → operations panel →
// plan preview → execute → TaskDetail approval → event timeline → succeeded.
//
// The stack under test is bootstrapped by e2e/stack.sh (webServer, N14): a
// dedicated schema, the real backend child with engine overrides, the kind
// v2-p3 fixture cluster registered through the same v1 row + sync-inventory
// CLI the N16 harness uses. Rollout convergence rides the pre-loaded
// nginx:1.27-alpine image (R12/H1) with minReadySeconds=10 (R2).
import { test, expect } from 'playwright/test'

const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'slicea-e2e-admin'
const OPERATION_NAME = 'k8s.workload.restart'
const WORKLOAD_NAME = 'restart-target'
// §0.7: recovery lower bound is ~36s; the terminal wait budget is 120s.
const TERMINAL_BUDGET = 120_000

test.describe.configure({ mode: 'serial' })

// Each test gets a fresh browser context, so both authenticate through the
// real login view first.
test.beforeEach(async ({ page }) => {
  // Pin the display locale so the assertions below read the en-US strings.
  await page.addInitScript((locale) => {
    window.localStorage.setItem('ops-admin-display-locale', locale)
  }, 'en-US')
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  // Login.vue renders the username input first and the password input second.
  await page.locator('.el-input input').first().fill('admin')
  await page.locator('input[type="password"]').fill(ADMIN_PASSWORD)
  await page.locator('button.submit-btn').click()
  await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 })
})

test('provider connections view lists the fixture cluster connection', async ({ page }) => {
  await page.goto('/infra/providers')
  await expect(page.locator('.page-title')).toHaveText('Provider Connections')
  const row = page.locator('.el-table__row', { hasText: 'kind-v2-p3' }).first()
  await expect(row).toBeVisible({ timeout: 20_000 })
})

test('run a restart operation through plan, approval and the event timeline', async ({ page }) => {
  // K8s Inventory: the fixture workload must be in the synced shadow inventory.
  // Filter to the workload kind — the unfiltered list paginates at 20 of ~41
  // rows, and the deployment rides past page 1 behind its pods.
  await page.goto('/infra/resources')
  await expect(page.locator('.page-title')).toHaveText('K8s Inventory')
  await page.locator('.el-select').first().click()
  await page.locator('.el-select-dropdown__item', { hasText: 'orchestration.workload' }).click()
  const row = page.locator('.el-table__row', { hasText: WORKLOAD_NAME })
    .filter({ hasText: 'orchestration.workload' })
    .first()
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.click()

  // Resource drawer → Operations tab (registry opdef × kind intersection, F-9).
  const drawer = page.locator('.el-drawer')
  await expect(drawer).toBeVisible()
  await drawer.locator('.el-tabs__item', { hasText: 'Operations' }).click()
  const opRow = drawer.locator('.el-table__row', { hasText: OPERATION_NAME }).first()
  await expect(opRow).toBeVisible({ timeout: 20_000 })

  // Plan preview is stateless (J1): it must answer before anything executes.
  await opRow.getByRole('button', { name: 'Run' }).click()
  const dialog = page.locator('.el-dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('Operation Plan Preview')
  await expect(dialog).toContainText(OPERATION_NAME)
  await dialog.getByRole('button', { name: 'Run' }).click()

  // Execute → 201 → the console routes to /infra/tasks?uid=… and opens the
  // TaskDetail drawer. The task lands awaiting_approval (J8).
  await expect(page).toHaveURL(/\/infra\/tasks/, { timeout: 20_000 })
  const taskUrl = page.url() // ?uid=… — the deep link reopens this task later
  const taskDrawer = page.locator('.el-drawer')
  await expect(taskDrawer).toBeVisible({ timeout: 20_000 })
  const statusTag = taskDrawer.locator('.el-descriptions .el-tag').first()
  await expect(statusTag).toHaveText('awaiting_approval', { timeout: 20_000 })

  // Approve (J8 posture: the button is enabled only while awaiting_approval)
  // then wait out the engine: queued → running → rollout → succeeded poll.
  const approve = taskDrawer.getByRole('button', { name: 'Approve' })
  await expect(approve).toBeEnabled({ timeout: 20_000 })
  await approve.click()
  await expect(statusTag).toHaveText('succeeded', { timeout: TERMINAL_BUDGET })

  // The §13.5 event chain is visible in the timeline (write order). Events
  // load on open — the 2s poll refreshes status only (plan §3.7) — so a real
  // user reopens the detail via the deep link to read the final timeline.
  // (KNOWN GAP, not worked around silently: the task LIST endpoint
  // GET /api/v2/infra/tasks is not registered backend-side — plan M8 only
  // wired tasks/:uid — so the list page itself stays empty; reported.)
  await taskDrawer.locator('button.drawer-close').click()
  await expect(taskDrawer).toBeHidden()
  await page.goto(taskUrl)
  await expect(taskDrawer).toBeVisible({ timeout: 20_000 })
  const eventTags = taskDrawer.locator('.event-timeline .el-tag')
  await expect(eventTags.filter({ hasText: 'approved' })).not.toHaveCount(0)
  await expect(eventTags.filter({ hasText: 'claimed' })).not.toHaveCount(0)
  await expect(eventTags.filter({ hasText: 'succeeded' })).not.toHaveCount(0)
})

// I10 J2 (§16.1): the console's create flow rides the connection-scoped
// k8s.resource.create operation — plan → execute(Idempotency-Key) → auto
// approve → poll — through the shared progress dialog. The dialog labels and
// the create dialog copy render through the main ko catalog (i18n.js `t`
// carries no en table), so the ko strings below are the stable selectors even
// under the pinned en-US locale.
test('create a namespace through the connection-scoped task flow and never replay a spent key', async ({ page }) => {
  await page.goto('/containers/k8s/namespaces')
  // The console auto-selects the first (only) fixture cluster — wait for its
  // namespace rows before driving the create dialog (submit is a no-op guard
  // while cluster.value is still unset).
  await expect(page.locator('.el-table__row').first()).toBeVisible({ timeout: 30_000 })
  const createButton = page.getByRole('button', { name: 'Namespace 생성' })
  await expect(createButton).toBeVisible()
  const executeResponse = () => page.waitForResponse(
    (response) => response.url().includes('/operations/k8s.resource.create/execute'),
    { timeout: 30_000 }
  )

  // First create: execute must return a real task uid, and the progress
  // dialog must carry the row to the succeeded terminal state. The namespace
  // name is unique per run (L-1): the kind cluster outlives the lane's
  // schema drop, so a fixed name would ride the executor's 409-convergent
  // leg forever — the fresh create needs a name that cannot pre-exist, and
  // a fresh submission answers 201 (a key replay would answer 200 instead).
  const nsName = `e2e-create-ns-${Date.now()}`
  await createButton.click()
  const dialog = page.locator('.el-dialog', { hasText: 'Namespace 생성' })
  await expect(dialog).toBeVisible()
  await dialog.locator('.el-input input').first().fill(nsName)
  const firstExecute = executeResponse()
  await dialog.getByRole('button', { name: '생성', exact: true }).click()
  const firstResponse = await firstExecute
  const uid1 = (await firstResponse.json())?.data?.task?.uid
  expect(uid1).toBeTruthy()
  expect(firstResponse.status()).toBe(201)
  const progress = page.locator('.el-dialog', { hasText: 'Resource Create Progress' })
  await expect(progress).toBeVisible()
  await expect(progress.locator('.el-tag').first()).toHaveText('Succeeded', { timeout: TERMINAL_BUDGET })

  // The progress dialog is modal — close it before re-firing through the
  // console header button.
  await progress.locator('.el-dialog__headerbtn').click()
  await expect(progress).toBeHidden({ timeout: 10_000 })

  // Re-fire the identical manifest after the terminal state: the burned key
  // must spawn a NEW task (uid2 ≠ uid1) — a replay of the spent key would
  // return the old task and its uid instead (silent false-success contract).
  await createButton.click()
  const redialog = page.locator('.el-dialog', { hasText: 'Namespace 생성' })
  await expect(redialog).toBeVisible()
  await redialog.locator('.el-input input').first().fill(nsName)
  const secondExecute = executeResponse()
  await redialog.getByRole('button', { name: '생성', exact: true }).click()
  const secondResponse = await secondExecute
  const uid2 = (await secondResponse.json())?.data?.task?.uid
  expect(uid2).toBeTruthy()
  // 201 again — the burned key forces a fresh submission; a lingering key
  // would replay 200 with the old task's uid.
  expect(secondResponse.status()).toBe(201)
  expect(uid2).not.toBe(uid1)
  await expect(progress.locator('.el-tag').first()).toHaveText('Succeeded', { timeout: TERMINAL_BUDGET })

  // The finish refresh pulls the created namespace into the cluster overview.
  await expect(page.locator('.el-table__row', { hasText: nsName }).first())
    .toBeVisible({ timeout: 30_000 })
})
