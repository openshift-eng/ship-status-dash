Feature: Outage Details
  As a dashboard user
  I want to view detailed information about an outage
  So that I can understand its severity, timeline, and resolution

  Scenario: Outage detail page shows key information
    Given I navigate to outage 1 for "prow/deck" as a guest
    Then I should see the severity "Degraded"
    And I should see the description "Deck UI is slow to respond"
    And the outage should show as unconfirmed
    And the outage end time should show "Not set"

  Scenario: Admin sees triage notes with existing content
    Given I am logged in as an admin for "Sippy"
    And I navigate to outage 2 for "sippy/sippy-ui" as an admin
    Then I should see the triage note "Investigating root cause"
    And I should be able to add a triage note

  Scenario: Outage links are visible with correct content
    Given I am logged in as an admin for "Sippy"
    And I navigate to outage 2 for "sippy/sippy-ui" as an admin
    Then I should see the outage link "Incident Channel/Thread"
    And I should be able to add an outage link

  Scenario: Audit log modal opens and shows change history
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck" as an admin
    When I click the "Audit Logs" button
    Then I should see the audit log modal
    And the audit log should show "create" and "update" entries
