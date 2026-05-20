import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 8080,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true
      },
      '/uploads': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true
      }
    }
  }
})
