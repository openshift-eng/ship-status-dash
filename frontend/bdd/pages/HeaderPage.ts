import type { Locator, Page } from '@playwright/test'

export class HeaderPage {
  readonly page: Page
  readonly loginButton: Locator
  readonly menuButton: Locator
  readonly logo: Locator

  constructor(page: Page) {
    this.page = page
    this.loginButton = page.locator('[data-tour="login-button"]')
    this.menuButton = page.getByRole('button', { name: 'Open menu' })
    this.logo = page.locator('header img[alt="Logo"]')
  }

  async openMenu() {
    await this.menuButton.click()
  }

  menuItem(label: string): Locator {
    return this.page.getByRole('menuitem', { name: label })
  }

  async navigateViaMenu(label: string) {
    await this.openMenu()
    await this.menuItem(label).click()
  }
}
