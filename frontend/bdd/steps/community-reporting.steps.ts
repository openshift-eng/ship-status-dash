import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { PROTECTED, PUBLIC, json, setupApiMocks } from '../fixtures/apiMocks'
import { mockAuthUser, mockComponents, mockSuspectedOutage } from '../fixtures/mockData'
import { SubComponentDetailsPage } from '../pages/SubComponentDetailsPage'

const { Given, When, Then } = createBdd()

Given('I am logged in but not an admin for {string}', async ({ page }, _component: string) => {
  await setupApiMocks(page, { authenticated: true })

  await page.route(`${PROTECTED}/api/user`, (route) => {
    return json(route, { username: mockAuthUser.username, components: [] })
  })
})

Given('I am on the {string} sub-component page', async ({ page }, path: string) => {
  const [compSlug, subSlug] = path.split('/')
  const subPage = new SubComponentDetailsPage(page)
  await subPage.goto(compSlug, subSlug)
  await subPage.container.waitFor()
})

Given(
  'I am logged in as a user who already reported on {string}',
  async ({ page }, _component: string) => {
    await setupApiMocks(page, { authenticated: true })

    await page.route(`${PROTECTED}/api/user`, (route) => {
      return json(route, { username: 'user1', components: [] })
    })
  },
)

Given('there is a suspected outage on {string}', async ({ page }, path: string) => {
  const [compSlug, subSlug] = path.split('/')

  const displayName = mockComponents.find((c) => c.slug === compSlug)?.name ?? compSlug

  await page.route(`${PUBLIC}/api/status/${compSlug}/${subSlug}`, (route) => {
    return json(route, {
      component_name: displayName,
      status: 'Suspected',
      active_outages: [],
      last_ping_time: new Date().toISOString(),
      suspected_outage: mockSuspectedOutage,
    })
  })
})

When('I submit the report dialog', async ({ page }) => {
  await page.getByRole('dialog').waitFor()
  await page.getByRole('button', { name: 'Submit' }).click()
})

Then('I should see the suspected outage banner', async ({ page }) => {
  await expect(page.getByText('Users are reporting an issue')).toBeVisible()
})

Then('the banner should show {string} reports', async ({ page }, count: string) => {
  await expect(page.getByText(`${count} reports`)).toBeVisible()
})

Then('the banner should show the description {string}', async ({ page }, description: string) => {
  await expect(page.getByText(description)).toBeVisible()
})

Then('I should see a disabled {string} button', async ({ page }, label: string) => {
  const button = page.getByRole('button', { name: label })
  await expect(button).toBeVisible()
  await expect(button).toBeDisabled()
})

Then('I should see a success notification', async ({ page }) => {
  await expect(page.getByRole('alert').filter({ hasText: /success|recorded/i })).toBeVisible()
})

Then('I should not see a {string} button', async ({ page }, label: string) => {
  const subPage = new SubComponentDetailsPage(page)
  await subPage.container.waitFor()
  await expect(page.getByRole('button', { name: label })).not.toBeVisible()
})
