Feature: Dashboard
  As a dashboard user
  I want to see all tracked components and their status
  So that I can quickly assess overall system health

  Scenario: All components are visible on the main page
    Given the dashboard API is available
    When I open the dashboard
    Then I should see component wells for "Prow, Build Farm, Sippy"

  Scenario: Components show their health status
    Given the dashboard API is available
    When I open the dashboard
    Then the "Prow" component well should show status "Degraded"
    And the "Build Farm" component well should show status "Healthy"
    And the "Sippy" component well should show status "Down"

  Scenario: Partial overall status renders on the home page
    Given the dashboard API is available
    And the "Prow" component has overall status "Partial"
    When I open the dashboard
    Then the "Prow" component well should show status "Partial"

  Scenario: Sub-components are listed within each component well
    Given the dashboard API is available
    When I open the dashboard
    Then the "Prow" component well should contain sub-components "Tide, Deck, Hook"

  Scenario: Unhealthy sub-components section appears when outages exist
    Given the dashboard API is available
    When I open the dashboard
    Then I should see the unhealthy sub-components section
    And the unhealthy well should contain sub-components "Deck, Sippy UI"
    And the ship logo should show the outage fire

  Scenario: Excluded sub-components do not appear in the In Outage well
    Given the dashboard API is available
    And an excluded unhealthy sub-component exists alongside other outages
    When I open the dashboard
    Then I should see the unhealthy sub-components section
    And the unhealthy well should contain sub-components "Deck, Sippy UI"
    And the unhealthy well should not contain sub-component "Incidents"
    And the ship logo should show the outage fire

  Scenario: Only excluded outages do not show the In Outage well or ship fire
    Given the dashboard API is available
    And the only unhealthy sub-component is excluded from the main outage well
    When I open the dashboard
    Then I should not see the unhealthy sub-components section
    And the ship logo should not show the outage fire
