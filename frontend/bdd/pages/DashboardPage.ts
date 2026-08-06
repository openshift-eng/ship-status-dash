import type { Locator, Page } from '@playwright/test'

export class DashboardPage {
  readonly page: Page
  readonly heading: Locator
  readonly componentList: Locator
  readonly componentWells: Locator
  readonly unhealthyWell: Locator
  readonly loadingSpinner: Locator
  readonly errorAlert: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.locator('[data-tour="home-heading"]')
    this.componentList = page.locator('[data-tour="component-list"]')
    this.componentWells = page.locator('[data-tour="component-well"]')
    this.unhealthyWell = page.locator('[data-tour="unhealthy-well"]')
    this.loadingSpinner = page.getByRole('progressbar')
    this.errorAlert = page.getByRole('alert')
  }

  async goto() {
    await this.page.goto('/')
  }

  componentWellByName(name: string): Locator {
    return this.componentWells.filter({ hasText: name })
  }

  subComponentCards(componentName: string): Locator {
    return this.componentWellByName(componentName).locator('[data-tour="subcomponent-card"]')
  }

  detailsButton(componentName: string): Locator {
    return this.componentWellByName(componentName).getByRole('button', { name: 'Details' })
  }
}
