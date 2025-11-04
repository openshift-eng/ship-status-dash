import { AccessTime, ArrowBack, Assignment, Info, Person, Settings } from '@mui/icons-material'
import {
    Alert,
    Box,
    Button,
    Chip,
    CircularProgress,
    Container,
    Paper,
    Typography,
    styled,
} from '@mui/material'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import type { Outage } from '../../types'
import { getOutageEndpoint } from '../../utils/endpoints'
import {
    formatDuration,
    getStatusBackgroundColor,
    getStatusChipColor,
    relativeTime,
} from '../../utils/helpers'
import { deslugify } from '../../utils/slugify'

import OutageActions from './actions/OutageActions'
import Field, { FieldBox, FieldLabel } from './OutageDetailsField'
import Section from './OutageDetailsSection'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const HeaderPaper = styled(Paper)<{ severity: string; resolved: boolean }>(({
  theme,
  severity,
  resolved,
}) => {
  const severityStatus = resolved ? 'Healthy' : severity
  const bgColor = getStatusBackgroundColor(theme, severityStatus)

  return {
    padding: theme.spacing(4),
    marginBottom: theme.spacing(3),
    borderRadius: theme.spacing(2),
    backgroundColor: bgColor,
    border: `2px solid ${bgColor}`,
    boxShadow: theme.shadows[4],
  }
})

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const PageTitle = styled(Typography)(() => ({
  fontWeight: 600,
  marginBottom: 8,
}))

const ChipSpacer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(0.5),
}))

const SeverityChip = styled(Chip)<{ severity: string }>(({ theme, severity }) => {
  const colorValue = getStatusChipColor(theme, severity)
  return {
    backgroundColor: colorValue,
    color: theme.palette.getContrastText(colorValue),
  }
})

const HeaderChip = styled(SeverityChip)(() => ({
  fontSize: '0.95rem',
  fontWeight: 600,
  height: 32,
}))

const ResolvedChip = styled(Chip)(() => ({
  fontSize: '0.95rem',
  fontWeight: 600,
  height: 32,
}))

const GridContainer = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: '1fr',
  gap: theme.spacing(3),
  [theme.breakpoints.up('md')]: {
    gridTemplateColumns: 'repeat(2, 1fr)',
  },
}))

const FullWidthGridItem = styled(Box)(({ theme }) => ({
  gridColumn: '1',
  [theme.breakpoints.up('md')]: {
    gridColumn: '1 / -1',
  },
}))

const SystemGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: '1fr',
  gap: theme.spacing(2),
  [theme.breakpoints.up('sm')]: {
    gridTemplateColumns: 'repeat(3, 1fr)',
  },
}))

const ConfirmationChipContainer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(0.5),
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  flexWrap: 'wrap',
}))

const HeaderContent = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  flexWrap: 'wrap',
  gap: theme.spacing(2),
}))

const HeaderTitleBox = styled(Box)(() => ({}))

const ErrorAlert = styled(Alert)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const LoadingContainer = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '400px',
}))

const TopActionsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  alignItems: 'center',
  '& button': {
    border: `1px solid ${theme.palette.divider}`,
  },
}))

const OutageDetailsPage = () => {
  const navigate = useNavigate()
  const { componentName, subComponentName, outageId } = useParams<{
    componentName: string
    subComponentName: string
    outageId: string
  }>()
  const [outage, setOutage] = useState<Outage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchOutage = useCallback(() => {
    if (!componentName || !subComponentName || !outageId) {
      setError('Missing component, subcomponent, or outage ID')
      setLoading(false)
      return
    }

    setLoading(true)
    setError(null)

    fetch(getOutageEndpoint(componentName, subComponentName, parseInt(outageId, 10)))
      .then((outageResponse) => {
        if (!outageResponse.ok) {
          // If 404, outage was deleted, navigate back
          if (outageResponse.status === 404) {
            if (componentName && subComponentName) {
              navigate(`/${componentName}/${subComponentName}`)
            } else {
              navigate('/')
            }
            return
          }
          throw new Error(`Failed to fetch outage: ${outageResponse.statusText}`)
        }
        return outageResponse.json()
      })
      .then((outageData) => {
        if (outageData) {
          setOutage(outageData)
        }
      })
      .catch((err) => {
        setError(err.message || 'Failed to fetch data')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponentName, outageId, navigate])

  useEffect(() => {
    fetchOutage()
  }, [fetchOutage])

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString)
    return `${date.toLocaleString()} (${relativeTime(date, new Date())})`
  }

  const formatNullableDateTime = (nullableTime: { Time: string; Valid: boolean }) => {
    if (!nullableTime.Valid) {
      return 'Not set'
    }
    return formatDateTime(nullableTime.Time)
  }

  const handleBack = () => {
    if (componentName && subComponentName) {
      navigate(`/${componentName}/${subComponentName}`)
    } else {
      navigate('/')
    }
  }

  const isResolved = () => {
    if (!outage?.end_time.Valid) {
      return false
    }
    const endTime = new Date(outage.end_time.Time)
    const now = new Date()
    return endTime < now
  }

  const getBackButtonLabel = () => {
    if (outage) {
      return `${deslugify(outage.component_name)} / ${deslugify(outage.sub_component_name)} Outages`
    }
    return 'Go Back'
  }

  if (loading) {
    return (
      <StyledContainer maxWidth="lg">
        <LoadingContainer>
          <CircularProgress />
        </LoadingContainer>
      </StyledContainer>
    )
  }

  if (error || !outage) {
    return (
      <StyledContainer maxWidth="lg">
        <ErrorAlert severity="error">{error || 'Outage not found'}</ErrorAlert>
        <BackButton onClick={handleBack} variant="contained" startIcon={<ArrowBack />}>
          {getBackButtonLabel()}
        </BackButton>
      </StyledContainer>
    )
  }

  return (
    <StyledContainer maxWidth="lg">
      <Box display="flex" justifyContent="space-between" alignItems="flex-start" marginBottom={3}>
        <BackButton onClick={handleBack} variant="outlined" startIcon={<ArrowBack />}>
          {getBackButtonLabel()}
        </BackButton>
        <TopActionsContainer>
          <OutageActions outage={outage} onSuccess={fetchOutage} onError={setError} />
        </TopActionsContainer>
      </Box>

      <HeaderPaper severity={outage.severity} resolved={isResolved()} elevation={2}>
        <HeaderContent>
          <HeaderTitleBox>
            <PageTitle variant="h4">Outage Details</PageTitle>
            <Typography variant="body1" color="text.secondary">
              {deslugify(outage.component_name)} / {deslugify(outage.sub_component_name)}
            </Typography>
          </HeaderTitleBox>
          {isResolved() ? (
            <ResolvedChip label="Resolved" color="success" size="medium" />
          ) : (
            <HeaderChip label={outage.severity} severity={outage.severity} size="medium" />
          )}
        </HeaderContent>
      </HeaderPaper>

      <GridContainer>
        <Section icon={<Info />} title="Basic Information">
          <Field label="Component" value={deslugify(outage.component_name)} />
          <Field label="Sub-Component" value={deslugify(outage.sub_component_name)} />
          <FieldBox>
            <FieldLabel variant="caption" color="text.secondary">
              Severity
            </FieldLabel>
            <ChipSpacer>
              <SeverityChip label={outage.severity} severity={outage.severity} size="small" />
            </ChipSpacer>
          </FieldBox>
          {outage.description && <Field label="Description" value={outage.description} />}
        </Section>

        <Section icon={<AccessTime />} title="Timing Information">
          <Field label="Start Time" value={formatDateTime(outage.start_time)} />
          <Field label="End Time" value={formatNullableDateTime(outage.end_time)} />
          <Field
            label="Duration"
            value={formatDuration(outage.start_time, outage.end_time)}
            valueVariant="primary"
          />
        </Section>

        <Section icon={<Person />} title="User Information">
          <Field label="Created By" value={outage.created_by} />
          {outage.resolved_by && <Field label="Resolved By" value={outage.resolved_by} />}
          {outage.confirmed_by && <Field label="Confirmed By" value={outage.confirmed_by} />}
        </Section>

        <Section icon={<Assignment />} title="Additional Information">
          <Field label="Discovered From" value={outage.discovered_from} />
          <FieldBox>
            <FieldLabel variant="caption" color="text.secondary">
              Confirmed
            </FieldLabel>
            <ConfirmationChipContainer>
              <Chip
                label={outage.confirmed_at.Valid ? 'Yes' : 'No'}
                color={outage.confirmed_at.Valid ? 'success' : 'default'}
                size="small"
              />
              {outage.confirmed_at.Valid && (
                <Typography variant="body2" color="text.secondary">
                  {formatDateTime(outage.confirmed_at.Time)}
                </Typography>
              )}
            </ConfirmationChipContainer>
          </FieldBox>
          {outage.triage_notes && (
            <Field label="Triage Notes" value={outage.triage_notes} valueVariant="pre-wrap" />
          )}
        </Section>

        <FullWidthGridItem>
          <Section icon={<Settings />} title="System Information">
            <SystemGrid>
              <Field label="Outage ID" value={outage.id} valueVariant="monospace" />
              <Field label="Created At" value={formatDateTime(outage.created_at)} />
              <Field label="Updated At" value={formatDateTime(outage.updated_at)} />
            </SystemGrid>
          </Section>
        </FullWidthGridItem>
      </GridContainer>
    </StyledContainer>
  )
}

export default OutageDetailsPage
