export type Status = 'Healthy' | 'Degraded' | 'Down' | 'Suspected' | 'Partial' | 'Unknown'

export interface ComponentStatus {
  component_name: string
  status: Status
  active_outages: Outage[]
  last_ping_time?: string
}

export interface Reason {
  ID: number
  CreatedAt: string
  UpdatedAt: string
  type: string
  check: string
  results: string
}

export interface SlackThread {
  channel: string
  thread_url: string
}

export interface Outage {
  ID: number
  CreatedAt: string
  UpdatedAt: string
  component_name: string
  sub_component_name: string
  severity: string
  start_time: string
  end_time: {
    Time: string
    Valid: boolean
  }
  auto_resolve: boolean
  description?: string
  discovered_from?: string
  created_by?: string
  resolved_by?: string
  confirmed_by?: string
  confirmed_at: {
    Time: string
    Valid: boolean
  }
  triage_notes?: string
  reasons?: Reason[]
  slack_threads?: SlackThread[]
}

export interface Monitoring {
  frequency: string
  component_monitor: string
  auto_resolve: boolean
}

export interface SlackReportingConfig {
  channel: string
  severity?: string
}

export interface SubComponent {
  name: string
  slug: string
  description: string
  managed: boolean
  requires_confirmation: boolean
  monitoring?: Monitoring
  slack_reporting?: SlackReportingConfig[]
  status?: Status
  active_outages?: Outage[]
}

export interface Component {
  name: string
  slug: string
  description: string
  ship_team: string
  slack_reporting?: SlackReportingConfig[]
  sub_components: SubComponent[]
  owners: Array<{
    rover_group?: string
    service_account?: string
    user?: string
  }>
  status?: string
  last_ping_time?: string
}
