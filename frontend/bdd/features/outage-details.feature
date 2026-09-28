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

  Scenario: Admin can view and add triage notes
    Given I am logged in as an admin for "Sippy"
    And I navigate to outage 2 for "sippy/sippy-ui" as an admin
    Then I should see the triage note "Investigating root cause"
    When I add a triage note "New triage note"
    Then I should see the triage note "New triage note"

  Scenario: Admin can view and add outage links
    Given I am logged in as an admin for "Sippy"
    And I navigate to outage 2 for "sippy/sippy-ui" as an admin
    Then I should see the outage link "Incident Channel/Thread"
    When I add an outage link "https://example.com/rca"
    Then I should see the outage link "RCA"

  Scenario: Guest can view related outages section
    Given I navigate to outage 2 for "sippy/sippy-ui" as a guest
    Then I should see the "Related Outages" section

  Scenario: Relationship link navigates to the related outage
    Given I navigate to outage 2 for "sippy/sippy-ui" as a guest
    Then I should see a relationship link "Caused by" pointing to "/prow/deck/outages/1"

  Scenario: Outage description renders Markdown formatting
    Given I navigate to outage 2 for "sippy/sippy-ui" as a guest
    Then I should see a rendered heading "Summary"
    And I should see a rendered link "incident report" pointing to "https://example.com/incident"
    And I should see a rendered code block containing "HTTP 500"
    And I should see rendered bold text "500 errors"

  Scenario: Triage note renders Markdown formatting
    Given I navigate to outage 2 for "sippy/sippy-ui" as a guest
    Then I should see rendered bold text "database logs"
    And I should see rendered inline code "connection timeout"

  Scenario: Plain-text description displays without Markdown
    Given I navigate to outage 1 for "prow/deck" as a guest
    Then I should see the description "Deck UI is slow to respond"

  Scenario: Unsafe HTML and scripts in descriptions do not execute
    Given I navigate to outage 3 for "prow/hook" as a guest
    Then I should not see a script tag on the page
    And I should see the safe text "Safe text here"
    And unsafe links should not be clickable

  Scenario: Audit log modal opens and shows change history
    Given I am logged in as an admin for "Prow"
    And I navigate to outage 1 for "prow/deck" as an admin
    When I click the "Audit Logs" button
    Then I should see the audit log modal
    And the audit log should show "create" and "update" entries
