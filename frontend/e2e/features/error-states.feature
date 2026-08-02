Feature: Error States
  As a dashboard user
  I want to see appropriate feedback when things go wrong
  So that I know when data is loading or an error has occurred

  Scenario: API failure shows an error message
    Given the API returns an error for components
    When I navigate to the dashboard
    Then I should see an error message

  Scenario: Loading state shows a spinner
    Given the API is slow to respond
    When I navigate to the dashboard
    Then I should see a loading spinner
