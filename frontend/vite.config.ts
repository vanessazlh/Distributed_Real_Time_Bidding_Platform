import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 3000,
    proxy: {
      // Proxy API and WebSocket calls to the Go notification service
      '/auctions': { target: 'http://localhost:8080', ws: true },
      '/auth':     { target: 'http://localhost:8080' },
      '/users':    { target: 'http://localhost:8080' },
      '/metrics':  { target: 'http://localhost:8080' },
    },
  },
})
