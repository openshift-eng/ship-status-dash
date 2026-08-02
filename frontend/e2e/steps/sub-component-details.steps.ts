import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { SubComponentDetailsPage } from '../pages/SubComponentDetailsPage'

const { Given, When, Then } = createBdd()

Given(
  'I navigate to the {string} sub-component page as a guest',
  async ({ page }, path: string) => {
    await setupApiMocks(page, { authenticated: false })
    const [compSlug, subSlug] = path.split('/')
    const subPage = new SubComponentDetailsPage(page)
    await subPage.goto(compSlug, subSlug)
    await subPage.container.waitFor()
  },
)

Then('I should see an outage grid', async ({ page }) => {
  const subPage = new SubComponentDetailsPage(page)
  await expect(subPage.outageGrid).toBeVisible()
})

Then(
  'the outage grid should show severity {string} and description {string}',
  async ({ page }, severity: string, description: string) => {
    const subPage = new SubComponentDetailsPage(page)
    await expect(subPage.outageGrid.getByText(severity)).toBeVisible()
    await expect(subPage.outageGrid.getByText(description)).toBeVisible()
  },
)

When('I filter by {string} outages', async ({ page }, label: string) => {
  const subPage = new SubComponentDetailsPage(page)
  await subPage.filterButton(label).click()
})

Then('the outage grid should show {string}', async ({ page }, text: string) => {
  const subPage = new SubComponentDetailsPage(page)
  await expect(subPage.outageGrid.getByText(text)).toBeVisible()
})

Then('the outage grid should not show {string}', async ({ page }, text: string) => {
  const subPage = new SubComponentDetailsPage(page)
  await expect(subPage.outageGrid.getByText(text)).not.toBeVisible()
})

When('I navigate to the parent component', async ({ page }) => {
  const subPage = new SubComponentDetailsPage(page)
  await subPage.componentLink.click()
})

When('I view the details of an outage', async ({ page }) => {
  const subPage = new SubComponentDetailsPage(page)
  await subPage.viewDetailsButton().first().click()
})

Then('I should be on an outage details page', async ({ page }) => {
  await expect(page).toHaveURL(/outages\/\d+/)
})
