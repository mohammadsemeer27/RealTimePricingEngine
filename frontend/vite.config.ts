import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  server: {
    port: 5173,
    proxy: {
      '/prices': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/readyz': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8082',
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
