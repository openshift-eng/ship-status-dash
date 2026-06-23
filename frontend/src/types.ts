export type Status =
  | 'Healthy'
  | 'Degraded'
  | 'Down'
  | 'Suspected'
  | 'Partial'
  | 'Unknown'
  | 'CapacityExhausted'

export interface SuspectedOutageInfo {
  outage_id: number
  report_count: number
  description?: string
  start_time: string
}

export interface ReportSuspectedOutageResponse {
  outage: Outage
  report_count: number
  created: boolean
}

export interface ComponentStatus {
  component_name: string
  status: Status
  active_outages: Outage[]
  last_ping_time?: string
  sub_component_statuses?: Record<string, Status>
  suspected_outage?: SuspectedOutageInfo
}

export interface OutageDayBucket {
  date: string // YYYY-MM-DD
  highest_severity: Status | null
  total_outage_minutes: number
  outage_count: number
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

export interface TriageNote {
  ID: number
  CreatedAt: string
  outage_id: number
  body: string
  author: string
}

export interface OutageLink {
  ID: number
  CreatedAt: string
  outage_id: number
  url: string
  link_type: 'incident_channel_thread' | 'rca' | 'other'
  description?: string
}

export interface OutageAuditLog {
  ID: number
  CreatedAt: string
  UpdatedAt: string
  outage_id: number
  user: string
  operation: string
  old?: string
  new?: string
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
  triage_notes?: TriageNote[]
  links?: OutageLink[]
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
  long_description?: string
  documentation_url?: string
  tags?: string[]
  requires_confirmation: boolean
  critical?: boolean
  monitoring?: Monitoring
  slack_reporting?: SlackReportingConfig[]
  status?: Status
  active_outages?: Outage[]
}

export interface SubComponentListItem extends SubComponent {
  component_name: string
}

export interface SubComponentListParams {
  componentName?: string
  tag?: string
  team?: string
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

export interface Tag {
  name: string
  description: string
  color: string
}
