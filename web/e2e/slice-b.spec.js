// Slice B UI-flow E2E (plan N17 — PR 30, gate ③ only).
//
// The stack under test is bootstrapped by e2e/stack.sh serve-b: a dedicated
// schema, the real backend child, and a MOCK aliyun cloud account seeded
// straight into the V2 tables (provider_connection → provider_context →
// infra_resource + resource_observation). No kind cluster and no real cloud
// credentials are involved — this is the E-1(b) interim posture (§13-10, the
// stack writes "interim": true on .stack/data/slice-b/artifact.json).
//
// Scope limit (plan §6, M6): this proves gate ③ — the UI reads the V2
// inventory through kindPrefix=compute. — and nothing else. Real-credential
// discovery (gate ①) and the §15 pairs (gate ②) are NOT covered by this run.
import { test, expect } from 'playwright/test'

const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'slicea-e2e-admin'

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
  await page.locator('.el-input input').first().fill('admin')
  await page.locator('input[type="password"]').fill(ADMIN_PASSWORD)
  await page.locator('button.submit-btn').click()
  await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 })
})

test('compute inventory lists the seeded mock VMs via kindPrefix=compute.', async ({ page }) => {
  // The view must actually issue the family filter — this is the M7/M12
  // wiring under proof, not just "some rows render".
  const listRequest = page.waitForRequest(
    (request) => request.url().includes('/api/v2/infra/resources') && request.url().includes('kindPrefix=compute.')
  )
  await page.goto('/infra/compute')
  await expect(page.locator('.page-title')).toHaveText('Compute Inventory')
  await listRequest

  // Both seeded compute.vm rows render.
  await expect(page.locator('.el-table__row', { hasText: 'mock-vm-web-01' })).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('.el-table__row', { hasText: 'mock-vm-db-01' })).toBeVisible({ timeout: 20_000 })

  // The seeded orchestration.node must NOT appear — the kindPrefix filter
  // (backend M7) keeps non-compute kinds out of the compute view.
  await expect(page.locator('.el-table__row', { hasText: 'mock-control-plane' })).toHaveCount(0)
})

test('compute resource detail drawer shows the normalized observation', async ({ page }) => {
  await page.goto('/infra/compute')
  const row = page.locator('.el-table__row', { hasText: 'mock-vm-web-01' })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.click()

  const drawer = page.locator('.el-drawer')
  await expect(drawer).toBeVisible()
  await expect(drawer).toContainText('urn:aliyun:ctx-mock-aliyun:compute.vm:i-mock0001')
  // The §8.2 latest observation rides the detail: normalized fields first,
  // then the raw provider payload — the K8sInventory drawer pattern.
  await expect(drawer).toContainText('cn-hangzhou')
  await expect(drawer).toContainText('ecs.g7.large')
  await expect(drawer).toContainText('InstanceId')
  await drawer.locator('button.drawer-close').click()
  await expect(drawer).toBeHidden()
})
