import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { DashboardPage } from '../pages/DashboardPage'

const { Given, When, Then } = createBdd()

Given('the dashboard API is available', async ({ page }) => {
  await setupApiMocks(page)
})

When('I open the dashboard', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await dashboard.goto()
  await dashboard.heading.waitFor()
})

Then('I should see component wells for {string}', async ({ page }, names: string) => {
  const dashboard = new DashboardPage(page)
  const componentNames = names.split(',').map((n) => n.trim())
  for (const name of componentNames) {
    await expect(dashboard.componentWellByName(name)).toBeVisible()
  }
})

Then(
  'the {string} component well should show status {string}',
  async ({ page }, name: string, status: string) => {
    const dashboard = new DashboardPage(page)
    const well = dashboard.componentWellByName(name)
    await expect(well.getByText(status, { exact: true }).first()).toBeVisible()
  },
)

Then(
  'the {string} component well should contain sub-components {string}',
  async ({ page }, componentName: string, subNames: string) => {
    const dashboard = new DashboardPage(page)
    const subComponentNames = subNames.split(',').map((n) => n.trim())
    for (const subName of subComponentNames) {
      await expect(
        dashboard.subComponentCards(componentName).filter({ hasText: subName }),
      ).toBeVisible()
    }
  },
)

Then('I should see the unhealthy sub-components section', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.unhealthyWell).toBeVisible()
})
