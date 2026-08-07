import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { DashboardPage } from '../pages/DashboardPage'
import { HeaderPage } from '../pages/HeaderPage'

const { Given, When, Then } = createBdd()

Given('I am on the main dashboard', async ({ page }) => {
  await setupApiMocks(page)
  const dashboard = new DashboardPage(page)
  await dashboard.goto()
  await dashboard.heading.waitFor()
})

When('I click the "Details" button on the {string} component', async ({ page }, name: string) => {
  const dashboard = new DashboardPage(page)
  await dashboard.detailsButton(name).click()
})

When(
  'I click the {string} sub-component card in the {string} component',
  async ({ page }, subName: string, componentName: string) => {
    const dashboard = new DashboardPage(page)
    const card = dashboard.subComponentCards(componentName).filter({ hasText: subName })
    await card.click()
  },
)

When('I navigate to {string} via the header menu', async ({ page }, label: string) => {
  const header = new HeaderPage(page)
  await header.navigateViaMenu(label)
})

When('I click the {string} tag', async ({ page }, tagName: string) => {
  await page.locator(`a[href="/tags/${tagName}"]`).first().click()
})

When('I navigate back', async ({ page }) => {
  await page.goBack()
})

Then('I should be on the {string} component details page', async ({ page }, slug: string) => {
  await expect(page).toHaveURL(new RegExp(`/${slug}$`))
})

Then('I should be on the {string} sub-component page', async ({ page }, path: string) => {
  await expect(page).toHaveURL(new RegExp(`/${path}$`))
})

Then('I should be on the status history page', async ({ page }) => {
  await expect(page).toHaveURL(/\/status-history$/)
})

Then('I should be on the tag page for {string}', async ({ page }, tag: string) => {
  await expect(page).toHaveURL(new RegExp(`/tags/${tag}$`))
})

Then('I should be on the main dashboard page', async ({ page }) => {
  await expect(page).toHaveURL(/\/$/)
})

Then('I should see incident history for {string}', async ({ page }, componentName: string) => {
  await expect(page.getByText(componentName, { exact: true }).first()).toBeVisible()
})

When('I click the {string} team chip', async ({ page }, teamName: string) => {
  await page.locator(`a[href="/team/${teamName}"]`).first().click()
})

Then('I should be on the team page for {string}', async ({ page }, team: string) => {
  await expect(page).toHaveURL(new RegExp(`/team/${team}$`))
})

Then('I should see the team heading {string}', async ({ page }, heading: string) => {
  await expect(page.getByText(heading)).toBeVisible()
})
