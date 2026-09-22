import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { PUBLIC, json, setupApiMocks } from '../fixtures/apiMocks'
import {
  mockComponentStatuses,
  mockExcludedUnhealthySubComponent,
  mockUnhealthySubComponents,
} from '../fixtures/mockData'
import { DashboardPage } from '../pages/DashboardPage'

const { Given, When, Then } = createBdd()

Given('the dashboard API is available', async ({ page }) => {
  await setupApiMocks(page)
})

Given('an excluded unhealthy sub-component exists alongside other outages', async ({ page }) => {
  await page.route(`${PUBLIC}/api/sub-components**`, (route) => {
    return json(route, [...mockUnhealthySubComponents, mockExcludedUnhealthySubComponent])
  })
})

Given(
  'the only unhealthy sub-component is excluded from the main outage well',
  async ({ page }) => {
    await page.route(`${PUBLIC}/api/sub-components**`, (route) => {
      return json(route, [mockExcludedUnhealthySubComponent])
    })
  },
)

Given(
  'the {string} component has overall status {string}',
  async ({ page }, componentName: string, status: string) => {
    await page.route(`${PUBLIC}/api/status`, (route) => {
      const url = new URL(route.request().url())
      if (url.pathname !== '/api/status') {
        return route.fallback()
      }
      return json(
        route,
        mockComponentStatuses.map((item) =>
          item.component_name === componentName ? { ...item, status } : item,
        ),
      )
    })
  },
)

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

Then('I should not see the unhealthy sub-components section', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.unhealthyWell).toHaveCount(0)
})

Then(
  'the unhealthy well should contain sub-components {string}',
  async ({ page }, names: string) => {
    const dashboard = new DashboardPage(page)
    const subComponentNames = names.split(',').map((n) => n.trim())
    for (const name of subComponentNames) {
      await expect(dashboard.unhealthyWellCard(name)).toBeVisible()
    }
  },
)

Then(
  'the unhealthy well should not contain sub-component {string}',
  async ({ page }, name: string) => {
    const dashboard = new DashboardPage(page)
    await expect(dashboard.unhealthyWellCard(name)).toHaveCount(0)
  },
)

Then('the ship logo should show the outage fire', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.shipLogo).toHaveAttribute('src', /logo-outage/)
})

Then('the ship logo should not show the outage fire', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await expect(dashboard.shipLogo).toHaveAttribute('src', /logo(?:-dark)?\.svg/)
  await expect(dashboard.shipLogo).not.toHaveAttribute('src', /logo-outage/)
})
