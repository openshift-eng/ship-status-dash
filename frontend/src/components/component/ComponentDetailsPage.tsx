import { ArrowBack } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Container,
  Divider,
  Paper,
  styled,
  Typography,
} from '@mui/material'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import useIntervalRefresh from '../../hooks/useIntervalRefresh'
import type { Component } from '../../types'
import { deferMountFetch } from '../../utils/deferMountFetch'
import { getComponentInfoEndpoint, getComponentStatusEndpoint } from '../../utils/endpoints'
import { formatStatusSeverityText, relativeTime } from '../../utils/helpers'
import { deslugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import { StatusChip } from '../StatusColors'
import SubComponentCard from '../sub-component/SubComponentCard'
import TeamChip from '../team/TeamChip'

const SERVICE_ACCOUNT_PREFIX = 'system:serviceaccount:'
const trimServiceAccountPrefix = (value: string): string =>
  value.startsWith(SERVICE_ACCOUNT_PREFIX) ? value.slice(SERVICE_ACCOUNT_PREFIX.length) : value

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const ComponentHeader = styled(Paper)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 2),
  padding: theme.spacing(4),
  marginBottom: theme.spacing(4),
  borderRadius: theme.spacing(2),
}))

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

const HeaderDivider = styled(Divider)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const InfoCardsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
  gap: theme.spacing(3),
}))

const SlackChannelsList = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(0.5),
}))

const SlackChannelItem = styled(Box)(({ theme }) => ({
  fontSize: theme.typography.pxToRem(14),
}))

const OwnersChipsWrap = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(1),
}))

const OwnerChipsRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(0.5),
}))

const ComponentDetailsPage = () => {
  const { componentSlug } = useParams<{ componentSlug: string }>()
  const navigate = useNavigate()
  const [component, setComponent] = useState<Component | null>(null)
  const [error, setError] = useState<string | null>(null)

  const validationError = !componentSlug ? 'Component name is required' : null
  const [loading, setLoading] = useState(!!componentSlug)
  const abortRef = useRef<AbortController | null>(null)

  const fetchComponent = useCallback(
    (silent: boolean) => {
      if (!componentSlug) {
        return
      }

      const componentName = deslugify(componentSlug)

      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      if (!silent) {
        setLoading(true)
        setError(null)
      }

      Promise.all([
        fetch(getComponentInfoEndpoint(componentName), { signal: controller.signal }).then(
          (res) => {
            if (!res.ok) {
              throw new Error(`Component "${componentName}" not found`)
            }
            return res.json()
          },
        ),
        fetch(getComponentStatusEndpoint(componentName), { signal: controller.signal }).then(
          (res) => {
            if (!res.ok) {
              throw new Error(`Failed to fetch status: ${res.statusText}`)
            }
            return res.json()
          },
        ),
      ])
        .then(([componentData, statusData]) => {
          if (controller.signal.aborted) {
            return
          }
          return {
            ...componentData,
            status: statusData.status || 'Unknown',
            last_ping_time: statusData.last_ping_time,
          }
        })
        .then((data) => {
          if (!data || controller.signal.aborted) {
            return
          }
          setComponent(data)
          if (silent) {
            setError(null)
          }
        })
        .catch((err) => {
          if (err instanceof DOMException && err.name === 'AbortError') {
            return
          }
          if (!silent) {
            setError(err instanceof Error ? err.message : 'Failed to fetch component details')
          }
        })
        .finally(() => {
          if (!controller.signal.aborted && !silent) {
            setLoading(false)
          }
        })
    },
    [componentSlug],
  )

  useEffect(() => {
    deferMountFetch(() => {
      fetchComponent(false)
    })
    return () => {
      abortRef.current?.abort()
    }
  }, [fetchComponent])

  useIntervalRefresh(() => fetchComponent(true), undefined, !!componentSlug)

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

      {(validationError || error) && <Alert severity="error">{validationError || error}</Alert>}

      {!component && !loading && !validationError && !error && (
        <Alert severity="error">Component not found</Alert>
      )}

      {component && !loading && !error && (
        <>
          <ComponentHeader elevation={2} status={component.status || 'Unknown'}>
            <HeaderContent>
              <Box>
                <ComponentTitle>{component.name}</ComponentTitle>
                <ComponentDescription>{component.description}</ComponentDescription>
              </Box>
              <StatusChip
                label={formatStatusSeverityText(component.status || 'Unknown')}
                status={component.status || 'Unknown'}
                variant="filled"
              />
            </HeaderContent>

            <HeaderDivider />

            <InfoCardsGrid>
              <InfoCard>
                <CardContent>
                  <InfoTitle>SHIP Team</InfoTitle>
                  {component.ship_team ? (
                    <TeamChip team={component.ship_team} />
                  ) : (
                    <InfoValue>No team specified</InfoValue>
                  )}
                </CardContent>
              </InfoCard>
              {component.slack_reporting && component.slack_reporting.length > 0 && (
                <InfoCard>
                  <CardContent>
                    <InfoTitle>Alerting Slack Channels</InfoTitle>
                    <InfoValue>
                      <SlackChannelsList>
                        {component.slack_reporting.map((config, index) => (
                          <SlackChannelItem key={index}>{config.channel}</SlackChannelItem>
                        ))}
                      </SlackChannelsList>
                    </InfoValue>
                  </CardContent>
                </InfoCard>
              )}
              <InfoCard>
                <CardContent>
                  <InfoTitle>Owners</InfoTitle>
                  <InfoValue>
                    {component.owners.length > 0 ? (
                      <OwnersChipsWrap>
                        {component.owners.map((owner, index) => {
                          const ownerItems: Array<{ label: string; value: string }> = []
                          if (owner.rover_group) {
                            ownerItems.push({ label: 'Rover Group', value: owner.rover_group })
                          }
                          if (owner.user) {
                            ownerItems.push({ label: 'User', value: owner.user })
                          }
                          if (owner.service_account) {
                            ownerItems.push({
                              label: 'Service Account',
                              value: trimServiceAccountPrefix(owner.service_account),
                            })
                          }

                          return (
                            <OwnerChipsRow key={index}>
                              {ownerItems.map((item, itemIndex) => (
                                <Chip
                                  key={itemIndex}
                                  label={`${item.label}: ${item.value}`}
                                  size="small"
                                  variant="outlined"
                                  sx={{ fontSize: '0.75rem' }}
                                />
                              ))}
                            </OwnerChipsRow>
                          )
                        })}
                      </OwnersChipsWrap>
                    ) : (
                      'No owners specified'
                    )}
                  </InfoValue>
                </CardContent>
              </InfoCard>
              {component.last_ping_time && (
                <InfoCard>
                  <CardContent>
                    <InfoTitle>Last Checked</InfoTitle>
                    <InfoValue>
                      {relativeTime(new Date(component.last_ping_time), new Date())}
                    </InfoValue>
                  </CardContent>
                </InfoCard>
              )}
            </InfoCardsGrid>
          </ComponentHeader>

          <SubComponentsSection>
            <SubComponentsTitle>Sub-components</SubComponentsTitle>
            <SubComponentsGrid>
              {component.sub_components.map((subComponent) => (
                <SubComponentCard
                  key={subComponent.name}
                  subComponent={subComponent}
                  componentName={component.name}
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
