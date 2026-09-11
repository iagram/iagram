import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The build lands in the Go module so `go build` embeds it. In dev, Vite
// serves the UI and proxies the API to a running `iagram up --no-open`.
export default defineConfig({
  plugins: [react()],
  build: { outDir: '../internal/web/dist', emptyOutDir: true },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:7777',
      '/icons': 'http://127.0.0.1:7777',
    },
  },
})
