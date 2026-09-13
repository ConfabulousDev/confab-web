import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@/components': path.resolve(__dirname, './src/components'),
      '@/pages': path.resolve(__dirname, './src/pages'),
      '@/services': path.resolve(__dirname, './src/services'),
      '@/types': path.resolve(__dirname, './src/types'),
      '@/hooks': path.resolve(__dirname, './src/hooks'),
      '@/utils': path.resolve(__dirname, './src/utils'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // Keep recharts in its own chunk to avoid circular dependency warnings
          recharts: ['recharts'],
          // Split eagerly-loaded vendor libraries out of the entry chunk (pfnq), one chunk
          // per library so a dependency bump only invalidates that library's cache entry.
          // Subpath entries must be listed explicitly; 'react-dom' alone does not pull in
          // the client renderer imported via 'react-dom/client'.
          react: ['react', 'react/jsx-runtime', 'react-dom', 'react-dom/client', 'scheduler'],
          'react-router': ['react-router', 'react-router-dom'],
          zod: ['zod'],
          tanstack: ['@tanstack/react-query', '@tanstack/query-core'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/auth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
