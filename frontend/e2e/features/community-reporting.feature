Feature: Community Issue Reporting
  As a non-admin user
  I want to report suspected issues
  So that the team is aware of potential problems

  Scenario: Non-admin user can report a suspected issue
    Given I am logged in but not an admin for "Build Farm"
    And I am on the "build-farm/registry" sub-component page
    When I click the "Report Issue" button
    And I submit the report dialog
    Then I should see a success notification

  Scenario: Suspected outage banner shows report count and description
    Given I am logged in but not an admin for "Prow"
    And there is a suspected outage on "prow/deck"
    When I navigate to the "prow/deck" sub-component page
    Then I should see the suspected outage banner
    And the banner should show "3" reports
    And the banner should show the description "Dashboard loading slowly"

  Scenario: User who already reported sees disabled button
    Given I am logged in as a user who already reported on "Prow"
    And there is a suspected outage on "prow/deck"
    When I navigate to the "prow/deck" sub-component page
    Then I should see a disabled "You reported this" button

  Scenario: Unauthenticated user does not see the Report Issue button
    Given I am not logged in
    When I navigate to the "build-farm/registry" sub-component page
    Then I should not see a "Report Issue" button
