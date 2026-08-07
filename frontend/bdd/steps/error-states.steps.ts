import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { PROTECTED, PUBLIC, json, setupApiMocks } from '../fixtures/apiMocks'
import { DashboardPage } from '../pages/DashboardPage'

const { Given, When, Then } = createBdd()

When('I navigate to the dashboard', async ({ page }) => {
  await page.goto('/')
})

Given('the API returns an error for components', async ({ page }) => {
  await setupApiMocks(page)
  await page.route(`${PUBLIC}/api/components`, (route) => json(route, { error: 'internal' }, 500))
  await page.route(`${PUBLIC}/api/status`, (route) => json(route, { error: 'internal' }, 500))
  await page.route(`${PUBLIC}/api/sub-components**`, (route) => json(route, []))
  await page.route(`${PUBLIC}/api/tags`, (route) => json(route, []))
  await page.route(`${PROTECTED}/api/user`, (route) => json(route, { error: 'unauthorized' }, 401))
})

Given('the API is slow to respond', async ({ page }) => {
  await setupApiMocks(page)
  await page.route(`${PUBLIC}/api/components`, async (route) => {
    await new Promise((r) => setTimeout(r, 5000))
    return json(route, [])
  })
  await page.route(`${PUBLIC}/api/status`, async (route) => {
    await new Promise((r) => setTimeout(r, 5000))
    return json(route, [])
  })
  await page.route(`${PUBLIC}/api/sub-components**`, (route) => json(route, []))
  await page.route(`${PUBLIC}/api/tags`, (route) => json(route, []))
  await page.route(`${PROTECTED}/api/user`, (route) => json(route, { error: 'unauthorized' }, 401))
})

Then('I should see an error message', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.errorAlert).toBeVisible()
})

Then('I should see a loading spinner', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.loadingSpinner).toBeVisible()
})
