export type Status = 'Healthy' | 'Degraded' | 'Down' | 'Suspected' | 'Partial' | 'Unknown'

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
}

export interface SubComponent {
  name: string
  description: string
  managed: boolean
  requires_confirmation: boolean
  status?: Status
  active_outages?: Outage[]
}

export interface Component {
  name: string
  description: string
  ship_team: string
  slack_channel: string
  sub_components: SubComponent[]
  owners: Array<{
    rover_group?: string
    service_account?: string
    user?: string
  }>
  status?: string
}
