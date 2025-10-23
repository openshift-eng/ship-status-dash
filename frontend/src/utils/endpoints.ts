import { slugify } from './slugify'

const getApiBaseUrl = () => {
  const baseUrl = process.env.REACT_APP_API_BASE_URL
  // If REACT_APP_API_BASE_URL is set, use it (for local development)
  // If not set, use relative URLs (for production where frontend and backend are served together)
  return baseUrl || ''
}

export const getComponentsEndpoint = () => `${getApiBaseUrl()}/api/components`

export const getComponentInfoEndpoint = (componentName: string) =>
  `${getApiBaseUrl()}/api/components/${slugify(componentName)}`

export const getOverallStatusEndpoint = () => `${getApiBaseUrl()}/api/status`

export const getSubComponentStatusEndpoint = (componentName: string, subComponentName: string) =>
  `${getApiBaseUrl()}/api/status/${slugify(componentName)}/${slugify(subComponentName)}`

export const getComponentStatusEndpoint = (componentName: string) =>
  `${getApiBaseUrl()}/api/status/${slugify(componentName)}`

export const createOutageEndpoint = (componentName: string, subComponentName: string) =>
  `${getApiBaseUrl()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`

export const modifyOutageEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getApiBaseUrl()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}`
