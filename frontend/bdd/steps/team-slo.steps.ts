import { expect } from '@playwright/test'
import { createBdd } from 'playwright-bdd'

import { installTRTSLOMocks, json, PROTECTED, setupApiMocks } from '../fixtures/apiMocks'
import {
  mockLongUnbrokenJobFailure,
  mockLongUnbrokenJobName,
  mockLongUnbrokenPayloadNote,
  mockTRTPayloadItem,
  mockTRTSLOUser,
} from '../fixtures/mockData'
import { DashboardPage } from '../pages/DashboardPage'
import { TeamSLOPage } from '../pages/TeamSLOPage'

const { Given, When, Then } = createBdd()

let savedBodies: Array<{ details?: { jobs?: Array<{ recurring_count?: number }> } }> = []
let linkCalls: string[] = []

Given('the TRT SLO is available', async ({ page }) => {
  await setupApiMocks(page)
  await installTRTSLOMocks(page)
})

Given('I am logged in as a TRT SLO owner', async ({ page }) => {
  await setupApiMocks(page, { authenticated: true })
  await page.route(`${PROTECTED}/api/user`, (route) => json(route, mockTRTSLOUser))
})

Given('the browser viewport is narrow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
})

When('I open the main dashboard', async ({ page }) => {
  const dashboard = new DashboardPage(page)
  await dashboard.goto()
  await dashboard.heading.waitFor()
})

When('I open the TRT SLO summary', async ({ page }) => {
  await page.getByText('TRT SLO', { exact: true }).click()
})

When('I open the TRT team SLO page', async ({ page }) => {
  await installTRTSLOMocks(page)
  const userResponse = page.waitForResponse((response) => response.url().includes('/api/user'))
  const teamPage = new TeamSLOPage(page)
  await teamPage.goto('TRT')
  await userResponse
  await teamPage.heading('TRT').waitFor()
})

When('I edit the payload and save', async ({ page }) => {
  savedBodies = []
  linkCalls = []
  await page.route(`${PROTECTED}/api/teams/**`, async (route) => {
    const url = route.request().url()
    const method = route.request().method()
    if (method === 'PUT' && url.endsWith('/slo/items')) {
      savedBodies.push(route.request().postDataJSON())
      return json(route, mockTRTPayloadItem)
    }
    linkCalls.push(`${method} ${url}`)
    return json(route, { error: 'unexpected' }, 500)
  })

  await page.getByRole('button', { name: 'Edit' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.waitFor()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeHidden()
})

Then('I should see {string}', async ({ page }, text: string) => {
  await expect(page.getByText(text, { exact: true }).first()).toBeVisible()
})

Then('the payload note should wrap within its value column', async ({ page }) => {
  const note = page.getByText(mockLongUnbrokenPayloadNote, { exact: true })
  await expect(note).toBeVisible()
  expect(await note.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true)
})

Then('the failed job name should be a linked heading', async ({ page }) => {
  const heading = page.getByRole('heading', { level: 4, name: mockLongUnbrokenJobName })
  await expect(heading).toBeVisible()
  await expect(heading.getByRole('link')).toHaveAttribute('href', 'https://example.com/payload/job')
})

Then('the failed job details should fit within the row', async ({ page }) => {
  const heading = page.getByRole('heading', { level: 4, name: mockLongUnbrokenJobName })
  const row = page.getByRole('listitem').filter({ has: heading })
  const failure = row.getByText(mockLongUnbrokenJobFailure, { exact: true })
  await expect(failure).toBeVisible()

  const fits = await row.evaluate(
    (element) =>
      element.scrollWidth <= element.clientWidth &&
      document.documentElement.scrollWidth <= document.documentElement.clientWidth,
  )
  expect(fits).toBe(true)

  const [rowBox, failureBox] = await Promise.all([row.boundingBox(), failure.boundingBox()])
  expect(rowBox).not.toBeNull()
  expect(failureBox).not.toBeNull()
  expect(failureBox!.width).toBeGreaterThan(rowBox!.width * 0.9)
})

Then('the failed job context links should be available', async ({ page }) => {
  await expect(page.getByText('3 Payload Streak', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Prow run' })).toHaveAttribute(
    'href',
    'https://example.com/payload/job/later-pass',
  )
  await expect(page.getByRole('button', { name: /Root cause C1:/ })).toBeVisible()
})

When('I collapse and expand the failed jobs', async ({ page }) => {
  await page.getByRole('button', { name: 'Failed blocking jobs' }).click()
  await expect(page.getByRole('heading', { level: 4, name: mockLongUnbrokenJobName })).toBeHidden()
  await page.getByRole('button', { name: '1 blocking job failed' }).click()
})

When('I select root cause {string}', async ({ page }, causeID: string) => {
  await page.getByRole('button', { name: new RegExp(`Root cause ${causeID}:`) }).click()
})

Then('root cause {string} should receive focus', async ({ page }, causeID: string) => {
  const cause = page.getByText(causeID, { exact: true }).last().locator('..').locator('..')
  await expect(cause).toBeFocused()
})

Then(
  'I should see the outage history button for {string}',
  async ({ page }, subComponent: string) => {
    await expect(
      page.getByRole('button', { name: `View ${subComponent} outage history` }),
    ).toBeVisible()
  },
)

When('I click the outage history button for {string}', async ({ page }, subComponent: string) => {
  await page.getByRole('button', { name: `View ${subComponent} outage history` }).click()
})

Then('I should be on the sub-component page for {string}', async ({ page }, path: string) => {
  await page.waitForURL(`**/${path}`)
})

Then('the payload save keeps recurring_count 3', async () => {
  expect(savedBodies).toHaveLength(1)
  expect(savedBodies[0].details?.jobs?.[0]?.recurring_count).toBe(3)
})

Then('the existing SLO link is not changed', async () => {
  expect(linkCalls).toEqual([])
})
