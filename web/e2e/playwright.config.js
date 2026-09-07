// Playwright UI-flow config (plan N14). The stack (dedicated schema, backend
// child, cluster registration + sync-inventory, vite dev server) is owned by
// web/e2e/stack.sh — webServer boots it before the run and reaps it after
// (plan J11: "스택 기동을 잡이 소유"). One worker: the trace mutates the single
// restart-target deployment and rides the per-resource active unique (§13.4).
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'playwright/test'

const thisDir = fileURLToPath(new URL('.', import.meta.url))
// The lane's own web port (stack.sh) — never the developer's 8080.
const webPort = process.env.E2E_WEB_PORT || '18080'
const baseURL = process.env.E2E_BASE_URL || `http://127.0.0.1:${webPort}`

export default defineConfig({
  // Relative paths resolve against this config file's directory (web/e2e).
  testDir: '.',
  timeout: 180_000, // one rollout: execute → approve → running → succeeded
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0, // risk R3 mitigation — one retry, same stack
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  outputDir: '.artifacts',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    locale: 'en-US'
  },
  webServer: {
    // Slice A ('serve', default) vs Slice B ('serve-b' — plan N17): the mode
    // picks the stack fixture (kind cluster vs DB-seeded mock cloud account).
    command: `bash stack.sh ${process.env.E2E_STACK_MODE || 'serve'}`,
    cwd: thisDir,
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 420_000, // first boot = schema DDL + migrations + go build (slow disks)
    stdout: 'pipe',
    stderr: 'pipe'
  }
})
