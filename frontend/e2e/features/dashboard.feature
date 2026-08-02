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

  Scenario: Sub-components are listed within each component well
    Given the dashboard API is available
    When I open the dashboard
    Then the "Prow" component well should contain sub-components "Tide, Deck, Hook"

  Scenario: Unhealthy sub-components section appears when outages exist
    Given the dashboard API is available
    When I open the dashboard
    Then I should see the unhealthy sub-components section
