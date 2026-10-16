import { defineConfig } from '@playwright/test'
import { defineBddConfig } from 'playwright-bdd'

const testDir = defineBddConfig({
  features: 'features/**/*.feature',
  steps: 'steps/**/*.ts',
})

export default defineConfig({
  testDir,
  timeout: 60_000,
  retries: 0,
  // Several local Chromium processes against one Vite server crash the browser (Target crashed).
  // CI keeps Playwright's default worker count.
  workers: process.env.CI ? undefined : 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:3030',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: 'npm run start',
    port: 3030,
    cwd: '..',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
})
