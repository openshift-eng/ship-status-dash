import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { ComponentDetailsPage } from '../pages/ComponentDetailsPage'

const { Given, When, Then } = createBdd()

Given('I navigate to the {string} component details page', async ({ page }, slug: string) => {
  await setupApiMocks(page)
  const detailsPage = new ComponentDetailsPage(page)
  await detailsPage.goto(slug)
  await detailsPage.subComponentCards.first().waitFor()
})

Then('I should see sub-component cards for {string}', async ({ page }, names: string) => {
  const detailsPage = new ComponentDetailsPage(page)
  const subNames = names.split(',').map((n) => n.trim())
  for (const name of subNames) {
    await expect(detailsPage.subComponentCardByName(name)).toBeVisible()
  }
})

Then(
  'the {string} sub-component card should show status {string}',
  async ({ page }, name: string, status: string) => {
    const detailsPage = new ComponentDetailsPage(page)
    const card = detailsPage.subComponentCardByName(name)
    await expect(card.getByText(status)).toBeVisible()
  },
)

When('I click the {string} sub-component card', async ({ page }, name: string) => {
  const detailsPage = new ComponentDetailsPage(page)
  await detailsPage.subComponentCardByName(name).click()
})
