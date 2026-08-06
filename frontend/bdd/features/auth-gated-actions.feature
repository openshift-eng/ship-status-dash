Feature: Auth-gated actions
  As a component maintainer
  I want outage management actions to only appear when I am authorized
  So that unauthorized users cannot trigger write operations

  Scenario: Authorized user sees the Report Outage button
    Given I am logged in as an admin for "Prow"
    When I navigate to the "prow/deck" sub-component page
    Then I should see the "Report Outage" button

  Scenario: Unauthenticated user does not see the Report Outage button
    Given I am not logged in
    When I navigate to the "prow/deck" sub-component page
    Then I should not see the "Report Outage" button

  Scenario: Admin can create an outage via the modal
    Given I am logged in as an admin for "Prow"
    And I am on the "prow/deck" sub-component page
    When I click the "Report Outage" button
    And I fill in the outage form and submit
    Then the "Report Outage" dialog should close

  Scenario: Admin sees all outage management actions
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck"
    When I open the outage actions menu
    Then I should see the "Resolve" action
    And I should see the "Delete" action
    And I should see the "Confirm" action

  Scenario: Admin can resolve an outage
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck"
    When I open the outage actions menu
    And I click the "Resolve" menu item
    And I confirm the "Resolve" dialog
    Then the "Resolve" dialog should close

  Scenario: Admin can delete an outage
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck"
    When I open the outage actions menu
    And I click the "Delete" menu item
    And I confirm the "Delete" dialog
    Then the "Delete" dialog should close

  Scenario: Admin can confirm an outage
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck"
    When I open the outage actions menu
    And I confirm the outage
    Then the outage details page should remain visible

  Scenario: Unauthenticated user does not see outage action buttons
    Given I am not logged in
    When I navigate to outage 1 for "prow/deck"
    Then I should not see outage action controls
