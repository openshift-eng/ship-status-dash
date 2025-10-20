import { slugify } from './slugify'

const getApiBaseUrl = () => process.env.REACT_APP_API_BASE_URL

export const getComponentsEndpoint = () => `${getApiBaseUrl()}/api/components`

export const getOverallStatusEndpoint = () => `${getApiBaseUrl()}/api/status`

export const getSubComponentStatusEndpoint = (componentName: string, subComponentName: string) =>
  `${getApiBaseUrl()}/api/status/${slugify(componentName)}/${slugify(subComponentName)}`

export const getComponentStatusEndpoint = (componentName: string) =>
  `${getApiBaseUrl()}/api/status/${slugify(componentName)}`

export const getSubComponentOutagesEndpoint = (componentName: string, subComponentName: string) =>
  `${getApiBaseUrl()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`
