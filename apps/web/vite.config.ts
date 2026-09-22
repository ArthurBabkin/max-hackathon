/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const here = (p: string) => fileURLToPath(new URL(p, import.meta.url))

export default defineConfig({
  plugins: [react()],

  resolve: {
    alias: {
      // Контракт и словарь текстов лежат в packages/ — они общие с бэкендом
      // и с ботом, поэтому копий внутри apps/web быть не должно.
      '@contract': here('../../packages/api-contract/index.ts'),
      '@texts': here('../../packages/shared/texts/texts.json'),
      '@': here('./src'),
    },
  },

  server: {
    // Vite по умолчанию не отдаёт файлы выше корня проекта, а нам нужны
    // packages/ — они на уровень выше apps/web.
    fs: { allow: [here('../..')] },
  },

  build: {
    // Мини-приложение открывается на 4G, бюджет по ТЗ §12 — 2,5 с.
    // Предупреждение должно срабатывать заметно раньше дефолтных 500 КБ.
    chunkSizeWarningLimit: 250,
    // Sourcemap нужен: внутри MAX нет консоли, и разбирать ошибку с телефона
    // без него нечем. На размер бандла .map не влияет — грузится по запросу.
    sourcemap: true,
  },

  test: {
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    // jsdom стоит секунды старта, и он нужен только компонентным тестам.
    // Логика в src/lib и src/voice гоняется в node и остаётся быстрой.
    environment: 'node',
    projects: [
      {
        extends: true,
        test: {
          name: 'unit',
          include: ['src/{lib,voice,api,ui}/**/*.test.ts'],
          environment: 'node',
        },
      },
      {
        extends: true,
        test: {
          name: 'dom',
          include: ['src/**/*.test.tsx'],
          environment: 'jsdom',
        },
      },
    ],
  },
})
