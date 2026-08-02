import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { setupApiMocks } from '../fixtures/apiMocks'
import { OutageDetailsPage } from '../pages/OutageDetailsPage'
import { SubComponentDetailsPage } from '../pages/SubComponentDetailsPage'

const { Given, When, Then } = createBdd()

Given('I am logged in as an admin for {string}', async ({ page }, _component: string) => {
  await setupApiMocks(page, { authenticated: true })
})

Given('I am not logged in', async ({ page }) => {
  await setupApiMocks(page, { authenticated: false })
})

When('I navigate to the {string} sub-component page', async ({ page }, path: string) => {
  const [compSlug, subSlug] = path.split('/')
  const subPage = new SubComponentDetailsPage(page)
  await subPage.goto(compSlug, subSlug)
  await subPage.container.waitFor()
})

When(
  'I navigate to outage {int} for {string}',
  async ({ page }, outageId: number, path: string) => {
    const [compSlug, subSlug] = path.split('/')
    const outagePage = new OutageDetailsPage(page)
    await outagePage.goto(compSlug, subSlug, outageId)
    await outagePage.header.waitFor()
  },
)

Then('I should see the {string} button', async ({ page }, label: string) => {
  await expect(page.getByRole('button', { name: label })).toBeVisible()
})

Then('I should not see the {string} button', async ({ page }, label: string) => {
  await expect(page.getByRole('button', { name: label })).not.toBeVisible()
})

When('I click the {string} button', async ({ page }, label: string) => {
  await page.getByRole('button', { name: label }).click()
})

Then('I should see the outage creation modal', async ({ page }) => {
  await expect(page.getByRole('dialog')).toBeVisible()
})

Then('I should not see outage action controls', async ({ page }) => {
  const outagePage = new OutageDetailsPage(page)
  await expect(outagePage.outageActions).not.toBeVisible()
})

When('I open the outage actions menu', async ({ page }) => {
  const outagePage = new OutageDetailsPage(page)
  await outagePage.openActionsMenu()
})

When('I click the {string} menu item', async ({ page }, label: string) => {
  await page.getByRole('menuitem', { name: label }).click()
})

When('I confirm the action dialog', async ({ page }) => {
  const dialog = page.getByRole('dialog')
  await dialog.waitFor()
  await dialog.getByRole('button', { name: /confirm|yes|resolve/i }).click()
})

Then('I should see the {string} action', async ({ page }, label: string) => {
  await expect(page.getByRole('menuitem', { name: label })).toBeVisible()
})

Then('I should see the resolve confirmation dialog', async ({ page }) => {
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Resolve' })).toBeVisible()
})
