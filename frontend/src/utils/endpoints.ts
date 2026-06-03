import type { SubComponentListParams } from '../types'

import { slugify } from './slugify'

export const getPublicDomain = () => {
  const envDomain = import.meta.env.VITE_PUBLIC_DOMAIN
  if (!envDomain) {
    throw new Error('VITE_PUBLIC_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getProtectedDomain = () => {
  const envDomain = import.meta.env.VITE_PROTECTED_DOMAIN
  if (!envDomain) {
    throw new Error('VITE_PROTECTED_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getComponentsEndpoint = () => `${getPublicDomain()}/api/components`

export const getTagsEndpoint = () => `${getPublicDomain()}/api/tags`

export const getComponentInfoEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}`

export const getOverallStatusEndpoint = () => `${getPublicDomain()}/api/status`

export const getSubComponentStatusEndpoint = (componentName: string, subComponentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}/${slugify(subComponentName)}`

export const getComponentStatusEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}`

export const getListSubComponentsEndpoint = (params: SubComponentListParams = {}) => {
  const search = new URLSearchParams()
  if (params.componentName) search.set('componentName', params.componentName)
  if (params.tag) search.set('tag', params.tag)
  if (params.team) search.set('team', params.team)
  const q = search.toString()
  return `${getPublicDomain()}/api/sub-components${q ? `?${q}` : ''}`
}

export const createOutageEndpoint = (componentName: string, subComponentName: string) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`

export const getSubComponentOutagesEndpoint = (componentName: string, subComponentName: string) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`

export const modifyOutageEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}`

export const getOutageEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}`

export const getOutageAuditLogsEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/audit-logs`

export const getOutagesDuringEndpoint = (
  componentName: string,
  subComponentName?: string,
  start?: Date,
  end?: Date,
) => {
  const params = new URLSearchParams()
  params.set('componentName', slugify(componentName))
  if (subComponentName) params.set('subComponentName', slugify(subComponentName))
  if (start) params.set('start', start.toISOString())
  if (end) params.set('end', end.toISOString())
  return `${getPublicDomain()}/api/outages/during?${params.toString()}`
}

export const getSubComponentHistoryEndpoint = (
  componentName: string,
  subComponentName: string,
  days: number,
) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outage-history?days=${days}`

export const triageNotesEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/triage-notes`

export const triageNoteEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
  noteId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/triage-notes/${noteId}`

export const outageLinksEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/links`

export const outageLinkEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
  linkId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/links/${linkId}`

export const getUserEndpoint = () => `${getProtectedDomain()}/api/user`

export const getExternalPageEndpoint = (slug: string) =>
  `${getPublicDomain()}/api/external-pages/${slug}`
