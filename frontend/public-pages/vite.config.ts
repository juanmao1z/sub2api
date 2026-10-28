import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from 'tailwindcss'
import autoprefixer from 'autoprefixer'
import baselineTailwind from './tailwind.config.js'

const root = fileURLToPath(new URL('.', import.meta.url))

export default defineConfig({
  root,
  base: './',
  // Root public assets (logo/community image) belong to the final application.
  publicDir: false,
  envDir: false,
  define: { 'import.meta.env.VITE_API_BASE_URL': JSON.stringify('/api/v1') },
  plugins: [vue({ template: { transformAssetUrls: { includeAbsolute: false } } })],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  css: {
    postcss: {
      plugins: [
        tailwindcss({
          ...baselineTailwind,
          content: [`${root}/index.html`, `${root}/src/**/*.{vue,js,ts}`],
        }),
        autoprefixer(),
      ],
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    manifest: true,
    target: 'es2020',
  },
})
