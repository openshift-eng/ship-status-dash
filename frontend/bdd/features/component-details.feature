Feature: Component Details
  As a dashboard user
  I want to view detailed information about a component
  So that I can see its sub-components and their statuses

  Scenario: Sub-component cards are listed with correct status
    Given I navigate to the "prow" component details page
    Then I should see sub-component cards for "Tide, Deck, Hook"
    And the "Deck" sub-component card should show status "Degraded"

  Scenario: Navigate to sub-component detail from component page
    Given I navigate to the "prow" component details page
    When I click the "Tide" sub-component card
    Then I should be on the "prow/tide" sub-component page
