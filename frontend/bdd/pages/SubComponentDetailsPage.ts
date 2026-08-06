import type { Locator, Page } from '@playwright/test'

export class SubComponentDetailsPage {
  readonly page: Page
  readonly container: Locator
  readonly header: Locator
  readonly filterToolbar: Locator
  readonly outageGrid: Locator
  readonly reportOutageButton: Locator
  readonly componentLink: Locator

  constructor(page: Page) {
    this.page = page
    this.container = page.locator('[data-tour="subcomponent-detail"]')
    this.header = page.locator('[data-tour="subcomponent-detail-header"]')
    this.filterToolbar = page.locator('[data-tour="subcomponent-detail-filter"]')
    this.outageGrid = page.locator('[data-tour="subcomponent-detail-grid"]')
    this.reportOutageButton = page.locator('[data-tour="subcomponent-report-outage"]')
    this.componentLink = page.locator('[data-tour="subcomponent-detail-component-link"]')
  }

  async goto(componentSlug: string, subComponentSlug: string) {
    await this.page.goto(`/${componentSlug}/${subComponentSlug}`)
  }

  filterButton(label: string): Locator {
    return this.filterToolbar.getByRole('button', { name: label })
  }

  viewDetailsButton(): Locator {
    return this.page.getByRole('button', { name: /view full details/i })
  }
}
