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

Then('I should see the {string} section', async ({ page }, sectionTitle: string) => {
  await expect(page.getByText(sectionTitle, { exact: true }).first()).toBeVisible()
})

Then(
  'I should see a relationship link {string} pointing to {string}',
  async ({ page }, label: string, href: string) => {
    const labelEl = page.getByText(label, { exact: true }).first()
    await expect(labelEl).toBeVisible()
    const link = labelEl.locator('..').getByRole('link')
    await expect(link).toHaveAttribute('href', href)
  },
)

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

Then('I should see a rendered heading {string}', async ({ page }, headingText: string) => {
  await expect(
    page.locator('[data-testid="markdown-content"] :is(h1,h2,h3,h4,h5,h6)', {
      hasText: headingText,
    }),
  ).toBeVisible()
})

Then(
  'I should see a rendered link {string} pointing to {string}',
  async ({ page }, linkText: string, href: string) => {
    const link = page.locator('[data-testid="markdown-content"] a', { hasText: linkText })
    await expect(link).toBeVisible()
    await expect(link).toHaveAttribute('href', href)
    await expect(link).toHaveAttribute('target', '_blank')
  },
)

Then(
  'I should see a rendered code block containing {string}',
  async ({ page }, codeText: string) => {
    await expect(
      page.locator('[data-testid="markdown-content"] pre code', { hasText: codeText }),
    ).toBeVisible()
  },
)

Then('I should see rendered bold text {string}', async ({ page }, text: string) => {
  await expect(
    page.locator('[data-testid="markdown-content"] strong', { hasText: text }),
  ).toBeVisible()
})

Then('I should see rendered inline code {string}', async ({ page }, text: string) => {
  const inlineCode = page.locator('[data-testid="markdown-content"] :not(pre) > code', {
    hasText: text,
  })
  await expect(inlineCode).toBeVisible()
})

Then('I should not see a script tag on the page', async ({ page }) => {
  const scriptTags = page.locator('[data-testid="markdown-content"] script')
  await expect(scriptTags).toHaveCount(0)
})

Then('I should see the safe text {string}', async ({ page }, text: string) => {
  await expect(page.getByText(text)).toBeVisible()
})

Then('unsafe links should not be clickable', async ({ page }) => {
  const jsLinks = page.locator('[data-testid="markdown-content"] a[href^="javascript:"]')
  await expect(jsLinks).toHaveCount(0)
})
