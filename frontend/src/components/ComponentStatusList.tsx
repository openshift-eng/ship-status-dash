import React, { useState, useEffect } from 'react'
import { Box, CircularProgress, Alert, Container, styled, Typography, Paper } from '@mui/material'
import { Component } from '../types'
import ComponentWell from './ComponentWell'
import { getComponentsEndpoint, getOverallStatusEndpoint } from '../utils/endpoints'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
}))

const LoadingBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

const TitleSection = styled(Box)(({ theme }) => ({
  padding: theme.spacing(3, 0),
  marginBottom: theme.spacing(4),
  textAlign: 'center',
  borderBottom: `2px solid ${theme.palette.divider}`,
}))

const MainTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '2rem',
  marginBottom: theme.spacing(1),
  color: theme.palette.text.primary,
  [theme.breakpoints.down('md')]: {
    fontSize: '1.75rem',
  },
  [theme.breakpoints.down('sm')]: {
    fontSize: '1.5rem',
  },
}))

const Subtitle = styled(Typography)(({ theme }) => ({
  fontSize: '1rem',
  color: theme.palette.text.secondary,
  fontWeight: 400,
}))

const ComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(3),
}))

const ComponentStatusList: React.FC = () => {
  const [components, setComponents] = useState<Component[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // Fetch components configuration and their statuses
    Promise.all([
      fetch(getComponentsEndpoint()).then((res) => res.json()),
      fetch(getOverallStatusEndpoint()).then((res) => res.json()),
    ])
      .then(([componentsData, statusesData]) => {
        // Create a map of component statuses for quick lookup
        const statusMap = new Map<string, string>()
        statusesData.forEach((status: any) => {
          statusMap.set(status.component_name, status.status)
        })

        // Combine components with their statuses
        const componentsWithStatuses = componentsData.map((component: Component) => ({
          ...component,
          status: statusMap.get(component.name) || 'Unknown',
        }))

        return componentsWithStatuses
      })
      .then((data) => {
        setComponents(data)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to fetch components')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [])

  if (loading) {
    return (
      <StyledContainer maxWidth="lg">
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      </StyledContainer>
    )
  }

  if (error) {
    return (
      <StyledContainer maxWidth="lg">
        <Alert severity="error">{error}</Alert>
      </StyledContainer>
    )
  }

  return (
    <StyledContainer maxWidth="lg">
      <TitleSection>
        <MainTitle>
          SHIP Status Dashboard
        </MainTitle>
        <Subtitle>
          Real-time monitoring of system components and availability
        </Subtitle>
      </TitleSection>

      <ComponentsGrid>
        {components.map((component) => (
          <ComponentWell key={component.name} component={component} />
        ))}
      </ComponentsGrid>
    </StyledContainer>
  )
}

export default ComponentStatusList
