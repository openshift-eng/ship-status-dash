import type { Locator, Page } from '@playwright/test'

export class OutageDetailsPage {
  readonly page: Page
  readonly header: Locator
  readonly outageActions: Locator
  readonly auditLogButton: Locator
  readonly triageNoteInput: Locator
  readonly addLinkButton: Locator

  constructor(page: Page) {
    this.page = page
    this.header = page.locator('[data-tour="outage-detail-header"]')
    this.outageActions = page.locator('[data-tour="outage-actions"]')
    this.auditLogButton = page.getByRole('button', { name: 'Audit Logs' })
    this.triageNoteInput = page.getByPlaceholder('Add a triage note...')
    this.addLinkButton = page.getByRole('button', { name: 'Add Link' })
  }

  async goto(componentSlug: string, subComponentSlug: string, outageId: number) {
    await this.page.goto(`/${componentSlug}/${subComponentSlug}/outages/${outageId}`)
  }

  confirmedStatus(): Locator {
    return this.page.getByText('Confirmed').locator('..').getByText('No')
  }

  async openActionsMenu() {
    await this.outageActions.click()
    await this.page.getByRole('menu').waitFor()
  }
}
