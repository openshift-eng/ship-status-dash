import { ArrowBack } from '@mui/icons-material'
import {
  Box,
  Container,
  Typography,
  Paper,
  Button,
  CircularProgress,
  Alert,
  styled,
  Card,
  CardContent,
  Divider,
} from '@mui/material'
import React, { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'

import type { Component } from '../types'
import { StatusChip } from './StatusColors'
import SubComponentCard from './SubComponentCard'
import { getComponentsEndpoint, getComponentStatusEndpoint } from '../utils/endpoints'
import { getStatusBackgroundColor } from '../utils/helpers'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const ComponentHeader = styled(Paper)<{ status: string }>(({ theme, status }) => {
  const color = getStatusBackgroundColor(theme, status)

  return {
    padding: theme.spacing(4),
    marginBottom: theme.spacing(4),
    borderRadius: theme.spacing(2),
    backgroundColor: color,
    border: `2px solid ${color}`,
  }
})

const HeaderContent = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: theme.spacing(3),
}))

const ComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '2.5rem',
  color: theme.palette.text.primary,
  [theme.breakpoints.down('md')]: {
    fontSize: '2rem',
  },
  [theme.breakpoints.down('sm')]: {
    fontSize: '1.75rem',
  },
}))

const ComponentDescription = styled(Typography)(({ theme }) => ({
  fontSize: '1.1rem',
  lineHeight: 1.6,
  color: theme.palette.text.secondary,
  marginBottom: theme.spacing(3),
}))

const InfoCard = styled(Card)(({ theme }) => ({
  height: '100%',
  borderRadius: theme.spacing(1.5),
}))

const InfoTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1rem',
  color: theme.palette.text.primary,
  marginBottom: theme.spacing(1),
}))

const InfoValue = styled(Typography)(({ theme }) => ({
  fontSize: '0.9rem',
  color: theme.palette.text.secondary,
}))

const SubComponentsSection = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(4),
}))

const SubComponentsTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1.5rem',
  color: theme.palette.text.primary,
  marginBottom: theme.spacing(3),
}))

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
  gap: theme.spacing(3),
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

const ComponentDetailsPage: React.FC = () => {
  const { componentName } = useParams<{ componentName: string }>()
  const navigate = useNavigate()
  const [component, setComponent] = useState<Component | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!componentName) {
      setError('Component name is required')
      setLoading(false)
      return
    }

    // Fetch component configuration and its specific status
    Promise.all([
      fetch(getComponentsEndpoint()).then((res) => res.json()),
      fetch(getComponentStatusEndpoint(componentName)).then((res) => res.json()),
    ])
      .then(([componentsData, statusData]) => {
        const foundComponent = componentsData.find((comp: Component) => comp.name === componentName)

        if (!foundComponent) {
          throw new Error(`Component "${componentName}" not found`)
        }

        // Add status to the component
        return {
          ...foundComponent,
          status: statusData.status || 'Unknown',
        }
      })
      .then((data) => {
        setComponent(data)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to fetch component details')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName])

  const handleBackClick = () => {
    navigate('/')
  }

  return (
    <StyledContainer maxWidth="lg">
      <BackButton variant="outlined" startIcon={<ArrowBack />} onClick={handleBackClick}>
        Main Dashboard
      </BackButton>

      {loading && (
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      )}

      {error && <Alert severity="error">{error}</Alert>}

      {!component && !loading && !error && <Alert severity="error">Component not found</Alert>}

      {component && !loading && !error && (
        <>
          <ComponentHeader elevation={2} status={component.status || 'Unknown'}>
            <HeaderContent>
              <Box>
                <ComponentTitle>{component.name}</ComponentTitle>
                <ComponentDescription>{component.description}</ComponentDescription>
              </Box>
              <StatusChip
                label={component.status || 'Unknown'}
                status={component.status || 'Unknown'}
                variant="filled"
              />
            </HeaderContent>

            <Divider sx={{ marginBottom: 3 }} />

            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
                gap: 3,
              }}
            >
              <InfoCard>
                <CardContent>
                  <InfoTitle>SHIP Team</InfoTitle>
                  <InfoValue>{component.ship_team}</InfoValue>
                </CardContent>
              </InfoCard>
              <InfoCard>
                <CardContent>
                  <InfoTitle>Alerting Slack Channel</InfoTitle>
                  <InfoValue>{component.slack_channel}</InfoValue>
                </CardContent>
              </InfoCard>
              <InfoCard>
                <CardContent>
                  <InfoTitle>Sub-components</InfoTitle>
                  <InfoValue>{component.sub_components.length}</InfoValue>
                </CardContent>
              </InfoCard>
              <InfoCard>
                <CardContent>
                  <InfoTitle>Owners</InfoTitle>
                  <InfoValue>
                    {component.owners.length > 0
                      ? component.owners.map((owner, index) => (
                          <Box key={index} sx={{ marginBottom: 0.5 }}>
                            {owner.rover_group && (
                              <Box component="span" sx={{ display: 'block' }}>
                                Rover Group: {owner.rover_group}
                              </Box>
                            )}
                            {owner.service_account && (
                              <Box component="span" sx={{ display: 'block' }}>
                                Service Account: {owner.service_account}
                              </Box>
                            )}
                          </Box>
                        ))
                      : 'No owners specified'}
                  </InfoValue>
                </CardContent>
              </InfoCard>
            </Box>
          </ComponentHeader>

          <SubComponentsSection>
            <SubComponentsTitle>
              Sub-components
            </SubComponentsTitle>
            <SubComponentsGrid>
              {component.sub_components.map((subComponent) => (
                <SubComponentCard
                  key={subComponent.name}
                  subComponent={subComponent}
                  componentName={component.name}
                  useBackgroundColor={true}
                />
              ))}
            </SubComponentsGrid>
          </SubComponentsSection>
        </>
      )}
    </StyledContainer>
  )
}

export default ComponentDetailsPage
