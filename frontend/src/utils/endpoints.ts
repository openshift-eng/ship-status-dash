import { slugify } from './slugify'

export const getPublicDomain = () => {
  const envDomain = process.env.REACT_APP_PUBLIC_DOMAIN
  if (!envDomain) {
    throw new Error('REACT_APP_PUBLIC_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getProtectedDomain = () => {
  const envDomain = process.env.REACT_APP_PROTECTED_DOMAIN
  if (!envDomain) {
    throw new Error('REACT_APP_PROTECTED_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getComponentsEndpoint = () => `${getPublicDomain()}/api/components`

export const getComponentInfoEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}`

export const getOverallStatusEndpoint = () => `${getPublicDomain()}/api/status`

export const getSubComponentStatusEndpoint = (componentName: string, subComponentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}/${slugify(subComponentName)}`

export const getComponentStatusEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}`

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
