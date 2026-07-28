# TRT-2794: BDD Tests for SHIP Status Dashboard Frontend

**Date:** 2026-07-26
**JIRA:** [TRT-2794](https://redhat.atlassian.net/browse/TRT-2794)
**Author:** Arnav Meduri

## Overview

Currently, there are no tests in place to verify the functionality of
the SHIP Status Dashboard frontend. The existing e2e tests in `test/e2e/`
test the Go backend API but never open a browser or render the React
app. Without frontend tests, there is no way to catch UX regressions as
new features (including AI-developed ones) are added.

BDD (Behavior-Driven Development) tests define expected frontend behavior
as user-facing scenarios in Given/When/Then format, for example: "Given I
navigate to the Prow component page, then I should see sub-components
Tide and Deck." Each scenario is both readable documentation and a
runnable test. Steps like "I open the dashboard" or "I click the Details
button" are written once and reused across scenarios, so new tests can
be added by composing existing steps without writing new browser
interaction code.

## Proposed Approach

### Framework

BDD scenarios are typically written in
[Gherkin](https://cucumber.io/docs/gherkin/reference/), a plain-text
format that uses Given/When/Then keywords. These are stored in `.feature`
files. A separate set of TypeScript files called step definitions maps each
step (e.g. "Given I open the dashboard") to a function that drives the
browser. A test runner executes the scenarios by calling the matching
step definitions in order.

To implement this, the design uses two tools:

- [Playwright](https://playwright.dev/docs/intro) is the test runner and
  browser automation library. It launches a browser (Chromium, Firefox,
  or WebKit), navigates to pages, clicks buttons, fills in forms, and
  checks that the expected text, elements, or states are present on the
  page. It automatically waits for elements to appear before interacting
  with them, runs tests in parallel across multiple workers, and
  generates an HTML report with screenshots and traces when tests fail.
- [`playwright-bdd`](https://vitalets.github.io/playwright-bdd/) connects
  Gherkin to Playwright. It reads `.feature` files and converts them into
  Playwright test files. The step definitions that map each
  Given/When/Then step to browser actions are written as TypeScript
  functions.

Unlike [Cucumber.js](https://github.com/cucumber/cucumber-js) (which can
also run Gherkin files but uses its own test runner), `playwright-bdd`
uses the Playwright test runner, which supports parallel execution, HTML
reporting, and visual regression testing
([`toHaveScreenshot()`](https://playwright.dev/docs/test-snapshots)).

### Test Data

The frontend fetches data from the Go backend via `VITE_PUBLIC_DOMAIN` and
`VITE_PROTECTED_DOMAIN`. For BDD tests, API responses are intercepted
using Playwright's [`page.route()`](https://playwright.dev/docs/mock) and
replaced with mock data. This means:

- Tests do not need the Go backend, PostgreSQL, or Prometheus to run.
- Each scenario controls exactly what the API returns, so tests are
  deterministic and not affected by backend state.
- Write actions (creating outages, reporting issues, etc.) are tested
  by intercepting the POST/PATCH/DELETE request and returning a mock
  response. The test can verify that the correct request was sent and
  that the UI updates after the response.
- Frontend rendering and interaction are tested on their own. Backend
  logic is already covered by the Go e2e tests.

Mock data is defined as TypeScript objects typed against the interfaces in
`frontend/src/types.ts` (`Component`, `Outage`, `SubComponent`, etc.), so
the compiler will flag mismatches if the API contract changes.

For auth-gated scenarios, the `/api/user` mock returns either a 401
(unauthenticated) or a user object with a `components` array that controls
admin access, matching how `AuthContext` works in production.

### Page Object Model

Each page has a Page Object class that wraps locators and interactions.
Step definitions call Page Object methods instead of using raw selectors.
The existing `data-tour` attributes in the codebase (`component-well`,
`subcomponent-card`, `subcomponent-detail`, etc.) provide stable selectors
that are already separate from CSS class names, making them good targets
for test locators.

### Directory Structure

```text
frontend/
  e2e/
    features/           # Gherkin .feature files
    steps/              # TypeScript step definitions
    fixtures/           # Route interception helpers and typed test data
    pages/              # Page Object classes
    playwright.config.ts
```

The `e2e/` directory lives inside `frontend/` to keep frontend test tooling
alongside the frontend code, consistent with where ESLint, Prettier, and
Vite config already live.

## Example Scenarios

The following examples show the pattern and scope of what will be tested.
Each feature area will have additional scenarios beyond what is shown here.

### Dashboard (main page)

```gherkin
Feature: Dashboard
  As a dashboard user
  I want to see all tracked components and their status
  So that I can quickly assess overall system health

  Scenario: All components are visible on the main page
    Given the API returns components "Prow, Build Farm, Sippy"
    When I open the dashboard
    Then I should see component wells for "Prow, Build Farm, Sippy"

  Scenario: Each component displays its name and status
    Given the API returns a component "Prow" with status "Healthy"
    When I open the dashboard
    Then the "Prow" component well should show status "Healthy"

  Scenario: Sub-components are listed within each component well
    Given the API returns a component "Prow" with sub-components "Tide, Deck, Hook"
    When I open the dashboard
    Then the "Prow" component well should contain sub-components "Tide, Deck, Hook"
```

### Navigation

```gherkin
Feature: Navigation
  As a dashboard user
  I want to navigate between pages using the header menu and links
  So that I can access different views of the dashboard

  Scenario: Clicking a component's Details button navigates to component details
    Given I am on the main dashboard
    When I click the "Details" button on the "Prow" component
    Then I should be on the "Prow" component details page

  Scenario: Clicking a sub-component card navigates to its details
    Given I am on the "Prow" component details page
    When I click the "Tide" sub-component card
    Then I should be on the "Prow / Tide" sub-component page
```

### Auth-gated Actions

```gherkin
Feature: Auth-gated actions
  As a component maintainer
  I want outage management actions to only appear when I am authorized
  So that unauthorized users cannot trigger write operations

  Scenario: Authorized user sees the Report Outage button
    Given I am logged in as an admin for "Prow"
    When I navigate to the "Prow / Tide" sub-component page
    Then I should see a "Report Outage" button

  Scenario: Unauthenticated user does not see the Report Outage button
    Given I am not logged in
    When I navigate to the "Prow / Tide" sub-component page
    Then I should not see a "Report Outage" button
```

## Planned Test Coverage

| Feature Area | What It Tests | Approx. Scenarios |
|---|---|---|
| Dashboard (`/`) | Each component is shown with its name, status, and sub-components. Unhealthy components appear in a summary at the top of the page. | 4-6 |
| Component details (`/:componentSlug`) | The page displays the component name, description, status, SHIP team, owners, and a grid of sub-component cards. | 4-5 |
| Sub-component details (`/:componentSlug/:subComponentSlug`) | The outage table shows severity, start/end times, and status. Outages can be filtered by ongoing/resolved and by date range. The "Report Outage" button is only visible to authorized users. | 5-7 |
| Outage actions | Admin users can create, update, resolve, delete, and confirm outages. Each action submits a request (mocked via `page.route()`), and the UI updates to reflect the change. | 5-7 |
| Outage details (`/.../:outageId`) | The page displays outage severity, confirmation status, start/end times, duration, description, triage notes, and attached links. Triage notes and links can be added, edited, and deleted by admin users. The audit log modal opens and displays a history of changes. | 6-8 |
| Community issue reporting | Non-admin users can report a suspected issue via the "Report Issue" button. The suspected outage banner shows the report count and description. Users who have already reported see a disabled button. | 3-4 |
| Navigation | Header menu links, "Details" buttons, sub-component cards, and outage rows all navigate to the correct page. Back buttons return to the previous page. | 4-5 |
| Theme | Toggling dark/light mode updates the page theme. Toggling accessibility mode updates contrast and colors. | 2-3 |
| Status history (`/status-history`) | Each sub-component shows an outage history bar. Clicking a sub-component name navigates to its details page. | 3-4 |
| Error states | When an API request fails, an error message is shown. While data is loading, a loading spinner is shown. | 2-3 |

## Implementation

### 1. Setup

Add `@playwright/test` and `playwright-bdd` as dev dependencies and
install the Chromium browser binary. Create the Playwright config at
`frontend/e2e/playwright.config.ts`, which does two things: connects
`.feature` files to step definitions using
[`defineBddConfig`](https://vitalets.github.io/playwright-bdd/#/configuration/index),
and automatically starts the Vite dev server before tests run using
[`webServer`](https://playwright.dev/docs/test-webserver). Add a
`test:e2e` script to `frontend/package.json` so tests can be run with
`npm run test:e2e`.

Set up the shared test infrastructure: a test data fixture that
intercepts API routes using `page.route()` and returns typed mock
responses, and Page Object classes for each page.

### 2. Core Scenarios

Write `.feature` files and step definitions for the feature areas listed
in the coverage table above: dashboard, component details, sub-component
details, outage actions, outage details, community issue reporting,
navigation, theme, status history, and error states.

### 3. Additional Coverage

After the core scenarios are written and passing, coverage can be
expanded to:

- **Tag and team pages** (`/tags/:tag`, `/team/:team`). These pages
  display a filtered list of sub-components and reuse the same
  components already tested on the dashboard, so they are lower
  priority.
- **Visual regression testing.** Playwright can take a screenshot of a
  page or element, save it as a baseline, and fail future runs if the
  screenshot changes
  ([`toHaveScreenshot()`](https://playwright.dev/docs/test-snapshots)).
  This catches visual changes that functional scenarios cannot, such
  as unintended changes to page layout or component appearance. These
  types of regressions might not be very applicable to this project
  since the UI uses standard Material UI components with minimal
  custom styling, but visual regression testing could be a useful
  addition if there is a need for it down the line.

### 4. CI Integration

The BDD tests will run in CI by adding the necessary Playwright browser
dependencies to the
[buildroot image](https://github.com/openshift-eng/ship-status-dash/blob/main/Dockerfile.buildroot).
The tests do not need the Go backend or database (API responses are
mocked), just Node.js and a browser. The buildroot already has Node.js;
the Playwright Chromium binary and its system library dependencies can
be installed by adding
[`npx playwright install --with-deps chromium`](https://playwright.dev/docs/ci)
to the Dockerfile.
