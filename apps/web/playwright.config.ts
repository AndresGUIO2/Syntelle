import { defineConfig } from '@playwright/test'

const browserProjects = [
  {
    name: 'chromium-desktop',
    use: {
      browserName: 'chromium' as const,
      viewport: { width: 1440, height: 1000 },
      colorScheme: 'light' as const,
    },
  },
  {
    name: 'chromium-mobile',
    use: {
      browserName: 'chromium' as const,
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
      colorScheme: 'dark' as const,
    },
  },
  {
    name: 'webkit-desktop',
    use: {
      browserName: 'webkit' as const,
      viewport: { width: 1440, height: 1000 },
      colorScheme: 'dark' as const,
    },
  },
  {
    name: 'webkit-mobile',
    use: {
      browserName: 'webkit' as const,
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
      colorScheme: 'light' as const,
    },
  },
]

export default defineConfig({
  testDir: './tests/e2e',
  outputDir: 'test-results/normal',
  timeout: 20_000,
  workers: 2,
  retries: 0,
  reporter: [
    ['list'],
    [
      'json',
      {
        outputFile: 'test-results/normal-results.json',
      },
    ],
  ],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:5173',
    trace: 'retain-on-failure',
  },
  projects: [
    ...browserProjects.map((project) => ({
      ...project,
      // Historical client/laboratory suites remain as evidence. Their browser
      // entry points are retired from the public application.
      testMatch: '**/backend-player.spec.ts',
    })),
    {
      name: 'api',
      testMatch: '**/api.spec.ts',
      use: { baseURL: process.env.PLAYWRIGHT_API_URL ?? 'http://127.0.0.1:8080' },
    },
  ],
})
