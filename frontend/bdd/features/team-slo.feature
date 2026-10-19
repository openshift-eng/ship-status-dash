Feature: Team SLO workspace
  As a dashboard user
  I want to see a team's payload SLO and edit it when I own that team
  So that status and incident context stay on the team page

  Scenario: Summary shows the incident and opens the team workspace
    Given the TRT SLO is available
    When I open the main dashboard
    Then I should see "TRT SLO"
    And I should see "CI payload rejected"
    When I open the TRT SLO summary
    Then I should be on the team page for "TRT"
    And I should see the team heading "TRT Dashboard"
    And I should see "nightly-rejected"
    And I should see "3 Payload Streak"

  Scenario: Team SLO owner sees add and edit
    Given I am logged in as a TRT SLO owner
    When I open the TRT team SLO page
    Then I should see the "Add payload" button
    And I should see an Edit button for "nightly-rejected"
    And I should see an Edit button for "nightly-states"

  Scenario: Component owner does not see SLO edit controls
    Given I am logged in as an admin for "Prow"
    When I open the TRT team SLO page
    Then I should not see the "Add payload" button
    And I should not see the "Edit" button

  Scenario: Logged-out user does not see SLO edit controls
    Given I am not logged in
    When I open the TRT team SLO page
    Then I should not see the "Add payload" button
    And I should not see the "Edit" button

  Scenario: Long unbroken payload note stays within its value column
    Given I am not logged in
    When I open the TRT team SLO page
    Then the payload note should wrap within its value column

  Scenario: Payload notes render Markdown safely
    Given I am not logged in
    When I open the TRT team SLO page with Markdown note examples
    Then the "nightly-markdown" payload note should render a link and list
    And the "nightly-markdown" payload note should render inline and fenced code
    And the "nightly-markdown" payload note should suppress unsafe URLs and raw HTML

  Scenario: Failed job details use the row width without narrow-screen overflow
    Given I am not logged in
    And the browser viewport is narrow
    When I open the TRT team SLO page
    Then the failed job name should be a linked heading
    And the failed job details should fit within the row
    And the failed job context links should be available
    When I collapse and expand the failed jobs
    And I select root cause "C1"
    Then root cause "C1" should receive focus

  Scenario: SLO component well has a history link to the sub-component page
    Given I am not logged in
    When I open the TRT team SLO page
    Then I should see the outage history button for "Incidents"
    When I click the outage history button for "Incidents"
    Then I should be on the sub-component page for "trt-incidents/incidents"

  Scenario: Home page SLO summary has a history link to the sub-component page
    Given the TRT SLO is available
    When I open the main dashboard
    Then I should see the outage history button for "Incidents"
    When I click the outage history button for "Incidents"
    Then I should be on the sub-component page for "trt-incidents/incidents"

  Scenario: Payload qualifiers use release-controller chip colors
    Given I am not logged in
    When I open the TRT team SLO page
    Then the "nightly-rejected" payload should show qualifier "dtk" as "Failed"
    And the "nightly-rejected" payload should show qualifier "fips" as "Passed"
    And the "nightly-states" payload should show qualifier "preview" as "Pending"
    And the "nightly-states" payload should show qualifier "sign" as "Accepted"
    And the "nightly-states" payload should show qualifier "gate" as "Rejected"

  Scenario: Editing a payload keeps the streak and existing link
    Given I am logged in as a TRT SLO owner
    When I open the TRT team SLO page
    And I edit the payload and save
    Then the payload save keeps recurring_count 3
    And the existing SLO link is not changed

  Scenario: Editing a qualifier updates the saved chip
    Given I am logged in as a TRT SLO owner
    When I open the TRT team SLO page
    And I change the "fips" qualifier state to "Failed" and save
    Then the payload save records qualifier "fips" as "Failed"
