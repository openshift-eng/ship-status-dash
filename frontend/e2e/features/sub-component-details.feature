Feature: Sub-component Details
  As a dashboard user
  I want to view outages for a sub-component
  So that I can understand its incident history

  Scenario: Outage grid shows severity and description from data
    Given I navigate to the "prow/deck" sub-component page as a guest
    Then I should see an outage grid
    And the outage grid should show severity "Degraded" and description "Deck UI is slow to respond"

  Scenario: Ongoing filter shows only active outages
    Given I navigate to the "prow/deck" sub-component page as a guest
    When I filter by "Ongoing" outages
    Then the outage grid should show "Deck UI is slow to respond"

  Scenario: Resolved filter hides active outages
    Given I navigate to the "prow/deck" sub-component page as a guest
    When I filter by "Resolved" outages
    Then the outage grid should not show "Deck UI is slow to respond"

  Scenario: Outage row click navigates to outage detail page
    Given I navigate to the "prow/deck" sub-component page as a guest
    When I view the details of an outage
    Then I should be on an outage details page

  Scenario: Navigate back to component from sub-component page
    Given I navigate to the "prow/deck" sub-component page as a guest
    When I navigate to the parent component
    Then I should be on the "prow" component details page
