import path from 'node:path'
import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

/** Browser OTLP → Jaeger (avoids CORS on :4318). See demo/docs/TRACING.md bước 6. */
const otlpProxy = {
  '/otel': {
    target: 'http://localhost:4318',
    changeOrigin: true,
    rewrite: (p: string) => p.replace(/^\/otel/, ''),
  },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: otlpProxy,
  },
  preview: {
    proxy: otlpProxy,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})
