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
      '@regions': here('../../packages/core/refdata/regions.json'),
      '@': here('./src'),
    },
  },

  server: {
    // Vite по умолчанию не отдаёт файлы выше корня проекта, а нам нужны
    // packages/ — они на уровень выше apps/web.
    fs: { allow: [here('../..')] },
    // Живой API в разработке (VITE_USE_MOCKS=off): тот же путь /api/v1, что
    // и за nginx в compose, поэтому запрос остаётся same-origin и preflight
    // с Authorization не возникает. Адрес меняется VITE_API_PROXY.
    proxy: {
      '/api/v1': { target: process.env.VITE_API_PROXY ?? 'http://localhost:8081' },
    },
  },

  build: {
    // MAX на iPhone работает с iOS 15.5, а цель Vite по умолчанию — Safari
    // 16.4: минификатор писал `@media (height<=700px)`, которое iOS 15 не
    // понимает, и на iPhone SE и 6s пропадали правила для низкого экрана.
    // Заодно приватные поля классов в библиотеках переписываются: в Safari 15
    // с ними есть ошибка движка. Цена — около 4 КБ бандла.
    target: ['chrome111', 'edge111', 'firefox114', 'safari15', 'ios15'],
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
          include: ['src/{lib,voice,api,ui,bridge}/**/*.test.ts'],
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
