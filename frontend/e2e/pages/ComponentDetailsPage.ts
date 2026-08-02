import type { Locator, Page } from '@playwright/test'

export class ComponentDetailsPage {
  readonly page: Page
  readonly subComponentCards: Locator

  constructor(page: Page) {
    this.page = page
    this.subComponentCards = page.locator('[data-tour="subcomponent-card"]')
  }

  async goto(componentSlug: string) {
    await this.page.goto(`/${componentSlug}`)
  }

  subComponentCardByName(name: string): Locator {
    return this.subComponentCards.filter({ hasText: name })
  }
}
