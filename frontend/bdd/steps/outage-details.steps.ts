import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { OutageDetailsPage } from '../pages/OutageDetailsPage'

const { Given, When, Then } = createBdd()

Given(
  'I navigate to outage {int} for {string} as a guest',
  async ({ page }, outageId: number, path: string) => {
    await setupApiMocks(page, { authenticated: false })
    const [compSlug, subSlug] = path.split('/')
    const outagePage = new OutageDetailsPage(page)
    await outagePage.goto(compSlug, subSlug, outageId)
    await outagePage.header.waitFor()
  },
)

Given(
  'I navigate to outage {int} for {string} as an admin',
  async ({ page }, outageId: number, path: string) => {
    const [compSlug, subSlug] = path.split('/')
    const outagePage = new OutageDetailsPage(page)
    await outagePage.goto(compSlug, subSlug, outageId)
    await outagePage.header.waitFor()
  },
)

Then('I should see the severity {string}', async ({ page }, severity: string) => {
  await expect(page.getByText(severity, { exact: true }).first()).toBeVisible()
})

Then('I should see the description {string}', async ({ page }, description: string) => {
  await expect(page.getByText(description)).toBeVisible()
})

Then('the outage should show as unconfirmed', async ({ page }) => {
  const outagePage = new OutageDetailsPage(page)
  await expect(outagePage.confirmedStatus()).toBeVisible()
})

Then('the outage end time should show {string}', async ({ page }, value: string) => {
  await expect(page.getByText(value)).toBeVisible()
})

Then('I should see the triage note {string}', async ({ page }, noteText: string) => {
  await expect(page.getByText(noteText)).toBeVisible()
})

When('I add a triage note {string}', async ({ page }, noteText: string) => {
  const outagePage = new OutageDetailsPage(page)
  await outagePage.triageNoteInput.fill(noteText)
  await page.getByRole('button', { name: 'Post Note' }).click()
})

Then('I should see the outage link {string}', async ({ page }, linkText: string) => {
  await expect(page.getByRole('link', { name: linkText })).toBeVisible()
})

When('I add an outage link {string}', async ({ page }, url: string) => {
  const outagePage = new OutageDetailsPage(page)
  await page.getByPlaceholder('https://...').fill(url)
  await outagePage.addLinkButton.click()
})

Then('I should see the audit log modal', async ({ page }) => {
  await expect(page.getByRole('dialog')).toBeVisible()
})

Then(
  'the audit log should show {string} and {string} entries',
  async ({ page }, entry1: string, entry2: string) => {
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText(entry1, { exact: false })).toBeVisible()
    await expect(dialog.getByText(entry2, { exact: false })).toBeVisible()
  },
)
