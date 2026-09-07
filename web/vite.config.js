import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The E2E lane (web/e2e/stack.sh) points the proxy at its own backend child
// via E2E_API_TARGET; local development keeps the 8082 default.
const apiTarget = process.env.E2E_API_TARGET || 'http://127.0.0.1:8082'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 8080,
    proxy: {
      '/api/v1': {
        target: apiTarget,
        changeOrigin: true,
        ws: true
      },
      // V2 infra surface (plan PR 23/25) rides the same backend — without this
      // proxy the dev server answers /api/v2/* itself with a 404.
      '/api/v2': {
        target: apiTarget,
        changeOrigin: true
      },
      '/uploads': {
        target: apiTarget,
        changeOrigin: true
      }
    }
  }
})
