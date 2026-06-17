import {
  CheckCircle,
  Clear,
  Error as ErrorIcon,
  OpenInNew,
  ReportProblem,
  Warning,
} from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Paper,
  Snackbar,
  styled,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import { DataGrid } from '@mui/x-data-grid'
import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'

import { useAuth } from '../../contexts/AuthContext'
import { useTags } from '../../contexts/TagsContext'
import type { ComponentStatus, Outage, ReportOutageResponse, SubComponent } from '../../types'
import {
  getComponentInfoEndpoint,
  getOutagesDuringEndpoint,
  getReportOutageEndpoint,
  getSubComponentOutagesEndpoint,
  getSubComponentStatusEndpoint,
} from '../../utils/endpoints'
import { formatStatusSeverityText, relativeTime } from '../../utils/helpers'
import { deslugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import OutageActions from '../outage/actions/OutageActions'
import UpsertOutageModal from '../outage/actions/UpsertOutageModal'
import OutageDetailsButton from '../outage/OutageDetailsButton'
import { SeverityChip } from '../StatusColors'
import TagChip from '../tags/TagChip'

import SuspectedReportsBanner from './SuspectedReportsBanner'

const HeaderBox = styled(Box)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 1),
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 24,
  padding: theme.spacing(2),
}))

const LoadingBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  padding: theme.spacing(4),
}))

const StyledPaper = styled(Paper)<{ status?: string }>(({ theme, status }) => ({
  padding: theme.spacing(3),
  marginBottom: theme.spacing(2),
  backgroundColor: theme.palette.background.paper,
  ...(status ? getStatusTintStyles(theme, status, 'inherit') : {}),
}))

const StyledButton = styled(Button)(({ theme }) => ({
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[800] : 'white',
  color: theme.palette.text.primary,
  '&:hover': {
    backgroundColor:
      theme.palette.mode === 'dark' ? theme.palette.grey[700] : theme.palette.grey[100],
  },
}))

const StyledDataGrid = styled(DataGrid)(({ theme }) => ({
  backgroundColor: theme.palette.background.paper,
  color: theme.palette.text.primary,
  '& .MuiDataGrid-main': {
    backgroundColor: theme.palette.background.paper,
  },
  '& .MuiDataGrid-columnHeaders': {
    backgroundColor: `${theme.palette.background.paper} !important`,
    color: theme.palette.text.primary,
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
  '& .MuiDataGrid-columnHeader': {
    backgroundColor: `${theme.palette.background.paper} !important`,
    color: theme.palette.text.primary,
  },
  '& .MuiDataGrid-columnHeaderTitle': {
    color: theme.palette.text.primary,
    fontWeight: 600,
  },
  '& .MuiDataGrid-cell': {
    borderBottom: `1px solid ${theme.palette.divider}`,
    color: theme.palette.text.primary,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  '& .MuiDataGrid-row:hover': {
    backgroundColor: theme.palette.action.hover,
  },
  '& .MuiDataGrid-footerContainer': {
    backgroundColor: theme.palette.background.default,
    color: theme.palette.text.primary,
  },
}))

const SubComponentDescription = styled(Typography)<{
  hasLongDescription?: boolean
  hasTags?: boolean
}>(({ theme, hasLongDescription, hasTags }) => ({
  marginBottom: hasLongDescription || hasTags ? theme.spacing(2) : 0,
}))

const SubComponentLongDescription = styled(Typography)<{
  hasDocumentation?: boolean
  hasTags?: boolean
}>(({ theme, hasDocumentation, hasTags }) => ({
  color: theme.palette.text.secondary,
  whiteSpace: 'pre-wrap',
  lineHeight: 1.6,
  marginBottom: hasDocumentation || hasTags ? theme.spacing(2) : 0,
}))

const DocumentationButtonContainer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(2),
}))

const TagsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(2),
}))

const PageContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const LastCheckedText = styled(Typography)(({ theme }) => ({
  marginTop: theme.spacing(1),
  opacity: 0.8,
}))

const HeaderActionsRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  gap: theme.spacing(2),
}))

const ReportOutageButton = styled(Button)(({ theme }) => ({
  backgroundColor: theme.palette.status.down.main,
  color: theme.palette.getContrastText(theme.palette.status.down.main),
  '&:hover': {
    backgroundColor: theme.palette.status.down.dark,
    color: theme.palette.getContrastText(theme.palette.status.down.dark),
  },
}))

const StatusDownIcon = styled(ErrorIcon)(({ theme }) => ({
  color: theme.palette.status.down.main,
}))

const StatusHealthyIcon = styled(CheckCircle)(({ theme }) => ({
  color: theme.palette.status.healthy.main,
}))

const StatusDegradedIcon = styled(Warning)(({ theme }) => ({
  color: theme.palette.status.degraded.main,
}))

const OngoingTimeText = styled(Typography)(({ theme }) => ({
  color: theme.palette.status.down.main,
}))

const ErrorAlert = styled(Alert)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const OutageFilterToolbar = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(2),
  marginBottom: theme.spacing(2),
}))

const DataGridContainer = styled(Box)({
  height: 600,
  width: '100%',
})

const SubComponentDetails = () => {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { componentSlug, subComponentSlug } = useParams<{
    componentSlug: string
    subComponentSlug: string
  }>()
  const { user, isComponentAdmin } = useAuth()
  const { getTag } = useTags()
  const [outages, setOutages] = useState<Outage[]>([])
  const [error, setError] = useState<string | null>(null)
  const [createOutageModalOpen, setCreateOutageModalOpen] = useState(false)
  const [reportDialogOpen, setReportDialogOpen] = useState(false)
  const [reportSubmitting, setReportSubmitting] = useState(false)
  const [reportDescription, setReportDescription] = useState('')
  const [reportError, setReportError] = useState<string | null>(null)
  const [reportSuccess, setReportSuccess] = useState<string | null>(null)
  const [subComponentStatus, setSubComponentStatus] = useState<ComponentStatus | null>(null)
  const [subComponent, setSubComponent] = useState<SubComponent | null>(null)
  const [statusFilter, setStatusFilter] = useState<'all' | 'ongoing' | 'resolved'>('all')

  const dateStart = searchParams.get('start')
  const dateEnd = searchParams.get('end')

  const componentName = componentSlug ? deslugify(componentSlug) : ''
  const subComponentName = subComponentSlug ? deslugify(subComponentSlug) : ''
  const isAdmin = isComponentAdmin(componentSlug || '')

  const validationError =
    !componentName || !subComponentName ? 'Missing component or subcomponent name' : null
  const [loading, setLoading] = useState(!!(componentName && subComponentName))

  const outagesEndpoint =
    dateStart && dateEnd
      ? getOutagesDuringEndpoint(
          componentName,
          subComponentName,
          new Date(dateStart + 'T00:00:00Z'),
          new Date(dateEnd + 'T23:59:59.999Z'),
        )
      : getSubComponentOutagesEndpoint(componentName, subComponentName)

  const fetchData = () => {
    if (!componentName || !subComponentName) {
      return
    }

    setTimeout(() => {
      setLoading(true)
      setError(null)

      Promise.all([
        fetch(outagesEndpoint),
        fetch(getSubComponentStatusEndpoint(componentName, subComponentName)),
        fetch(getComponentInfoEndpoint(componentName)),
      ])
        .then(([outagesResponse, statusResponse, componentResponse]) => {
          if (!outagesResponse.ok) {
            setError(`Failed to fetch outages: ${outagesResponse.statusText}`)
            return
          }
          if (!statusResponse.ok) {
            setError(`Failed to fetch status: ${statusResponse.statusText}`)
            return
          }
          if (!componentResponse.ok) {
            setError(`Failed to fetch component: ${componentResponse.statusText}`)
            return
          }
          return Promise.all([
            outagesResponse.json(),
            statusResponse.json(),
            componentResponse.json(),
          ])
        })
        .then((results) => {
          if (results) {
            const [outagesData, statusData, componentData] = results
            if (outagesData) {
              setOutages(outagesData)
              if (!dateStart && !dateEnd) {
                const hasOngoing = outagesData.some((outage: Outage) => !outage.end_time.Valid)
                setStatusFilter(hasOngoing ? 'ongoing' : 'all')
              }
            }
            if (statusData) {
              setSubComponentStatus(statusData)
            }
            if (componentData) {
              const foundSubComponent = componentData.sub_components.find(
                (sub: SubComponent) => sub.slug === subComponentSlug,
              )
              if (foundSubComponent) {
                setSubComponent(foundSubComponent)
              }
            }
          }
        })
        .catch(() => {
          setError('Failed to fetch data')
        })
        .finally(() => {
          setLoading(false)
        })
    }, 0)
  }

  useEffect(() => {
    if (!componentName || !subComponentName) {
      return
    }
    fetchData()
  }, [componentName, subComponentName, subComponentSlug, outagesEndpoint, dateStart, dateEnd])

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  const getStatusText = (outage: Outage) => {
    if (outage.end_time.Valid) {
      return 'Resolved'
    }
    return 'Active'
  }

  const handleOutageAction = () => {
    fetchData()
  }

  const handleReportSubmit = () => {
    if (!componentName || !subComponentName) return
    setReportSubmitting(true)
    const body: Record<string, string> = {}
    const trimmed = reportDescription.trim()
    if (trimmed) {
      body.description = trimmed
    }
    setReportError(null)
    fetch(getReportOutageEndpoint(componentName, subComponentName), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(body),
    })
      .then((response) => {
        if (response.ok) {
          return response.json().then((data: ReportOutageResponse) => {
            const count = data.report_count
            setReportSuccess(
              `Report recorded \u2014 ${count} ${count === 1 ? 'report' : 'reports'}`,
            )
            setReportDialogOpen(false)
            setReportDescription('')
            fetchData()
          })
        } else {
          return response.json().then((data: { error?: string }) => {
            setReportError(data.error || 'Failed to submit report')
          })
        }
      })
      .catch(() => {
        setReportError('Failed to submit report')
      })
      .finally(() => {
        setReportSubmitting(false)
      })
  }

  const columns: GridColDef[] = [
    {
      field: 'status',
      headerName: 'Status',
      width: 80,
      renderCell: (params) => {
        const outage = params.row as Outage
        const status = getStatusText(outage)
        const isActive = status === 'Active'

        return (
          <Tooltip title={status} arrow>
            {isActive ? <StatusDownIcon /> : <StatusHealthyIcon />}
          </Tooltip>
        )
      },
    },
    {
      field: 'severity',
      headerName: 'Severity',
      width: 120,
      renderCell: (params) => (
        <SeverityChip
          label={formatStatusSeverityText(params.value)}
          severity={params.value}
          size="small"
          variant="outlined"
        />
      ),
    },
    ...(subComponent?.requires_confirmation
      ? [
          {
            field: 'confirmation',
            headerName: 'Confirmation',
            width: 120,
            sortable: false,
            filterable: false,
            renderCell: (params: GridRenderCellParams) => {
              const outage = params.row as Outage
              const isConfirmed = outage.confirmed_at.Valid

              return (
                <Tooltip title={isConfirmed ? 'Confirmed' : 'Unconfirmed'} arrow>
                  {isConfirmed ? <StatusHealthyIcon /> : <StatusDegradedIcon />}
                </Tooltip>
              )
            },
          },
        ]
      : []),
    {
      field: 'description',
      headerName: 'Description',
      flex: 1,
      minWidth: 200,
      renderCell: (params) => (
        <Typography variant="body2" noWrap title={params.value || 'No description'}>
          {params.value || 'No description'}
        </Typography>
      ),
    },
    {
      field: 'start_time',
      headerName: 'Start Time',
      width: 120,
      renderCell: (params) => {
        const startDate = new Date(params.value)
        const now = new Date()
        const relative = relativeTime(startDate, now)
        return (
          <Typography variant="body2" title={formatDate(params.value)}>
            {relative}
          </Typography>
        )
      },
    },
    {
      field: 'end_time',
      headerName: 'End Time',
      width: 120,
      renderCell: (params) => {
        const outage = params.row as Outage
        if (outage.end_time.Valid) {
          const endDate = new Date(outage.end_time.Time)
          const now = new Date()
          const relative = relativeTime(endDate, now)
          return (
            <Typography variant="body2" title={formatDate(outage.end_time.Time)}>
              {relative}
            </Typography>
          )
        }
        return <OngoingTimeText variant="body2">Ongoing</OngoingTimeText>
      },
    },
    {
      field: 'details',
      headerName: 'Details',
      width: 100,
      sortable: false,
      filterable: false,
      renderCell: (params) => {
        const outage = params.row as Outage
        return <OutageDetailsButton outage={outage} />
      },
    },
    ...(isAdmin
      ? [
          {
            field: 'actions',
            headerName: 'Actions',
            width: 100,
            sortable: false,
            filterable: false,
            renderCell: (params: GridRenderCellParams) => {
              const outage = params.row as Outage
              return (
                <OutageActions outage={outage} onSuccess={handleOutageAction} onError={setError} />
              )
            },
          },
        ]
      : []),
  ]

  const filteredOutages = outages.filter((outage) => {
    if (statusFilter === 'ongoing') {
      return !outage.end_time.Valid
    }
    if (statusFilter === 'resolved') {
      return outage.end_time.Valid
    }
    return true // 'all'
  })

  // Sort outages: active first, then by start time descending
  const sortedOutages = [...filteredOutages].sort((a, b) => {
    const aActive = !a.end_time.Valid
    const bActive = !b.end_time.Valid

    if (aActive && !bActive) return -1
    if (!aActive && bActive) return 1

    return new Date(b.start_time).getTime() - new Date(a.start_time).getTime()
  })

  const handleFilterChange = (
    _event: React.MouseEvent<HTMLElement>,
    newFilter: 'all' | 'ongoing' | 'resolved' | null,
  ) => {
    if (newFilter !== null) {
      setStatusFilter(newFilter)
    }
  }

  const handleDateChange = (field: 'start' | 'end', value: string) => {
    const params = new URLSearchParams(searchParams)
    if (value) {
      params.set(field, value)
    } else {
      params.delete(field)
    }
    setSearchParams(params)
  }

  const clearDateFilter = () => {
    setSearchParams({})
  }

  if (!componentName || !subComponentName) {
    return (
      <PageContainer maxWidth="xl" data-tour="subcomponent-detail">
        <Alert severity="error">Invalid component or subcomponent</Alert>
      </PageContainer>
    )
  }

  return (
    <PageContainer maxWidth="xl" data-tour="subcomponent-detail">
      <StyledPaper status={subComponentStatus?.status || 'Unknown'}>
        <HeaderBox
          status={subComponentStatus?.status || 'Unknown'}
          data-tour="subcomponent-detail-header"
        >
          <Box>
            <Typography variant="h4">
              {componentName} / {subComponentName} - Outages
            </Typography>
            {subComponentStatus?.last_ping_time && subComponent?.monitoring?.frequency && (
              <LastCheckedText variant="body2">
                Last Checked:{' '}
                {relativeTime(new Date(subComponentStatus.last_ping_time), new Date())} · Expected
                Frequency: {subComponent.monitoring.frequency}
              </LastCheckedText>
            )}
          </Box>
          <HeaderActionsRow>
            {isAdmin && (
              <ReportOutageButton
                variant="contained"
                startIcon={<ReportProblem />}
                onClick={() => setCreateOutageModalOpen(true)}
                data-tour="subcomponent-report-outage"
              >
                Report Outage
              </ReportOutageButton>
            )}
            {user && !isAdmin && !subComponentStatus?.suspected_outage && (
              <Button
                variant="outlined"
                startIcon={<ReportProblem />}
                onClick={() => setReportDialogOpen(true)}
              >
                Report Issue
              </Button>
            )}
            <StyledButton
              variant="contained"
              onClick={() => navigate(`/${componentSlug}`)}
              data-tour="subcomponent-detail-component-link"
            >
              {componentName} Details
            </StyledButton>
          </HeaderActionsRow>
        </HeaderBox>
      </StyledPaper>

      {(subComponent?.description ||
        subComponent?.long_description ||
        subComponent?.documentation_url ||
        (subComponent?.tags && subComponent.tags.length > 0)) && (
        <StyledPaper>
          {subComponent?.description && (
            <SubComponentDescription
              variant="body1"
              hasLongDescription={!!subComponent?.long_description}
              hasTags={!!(subComponent?.tags && subComponent.tags.length > 0)}
            >
              {subComponent.description}
            </SubComponentDescription>
          )}
          {subComponent?.tags && subComponent.tags.length > 0 && (
            <TagsContainer>
              {subComponent.tags.map((tag) => (
                <TagChip key={tag} tag={tag} size="small" color={getTag(tag)?.color} />
              ))}
            </TagsContainer>
          )}
          {subComponent?.long_description && (
            <SubComponentLongDescription
              variant="body2"
              hasDocumentation={!!subComponent?.documentation_url}
              hasTags={!!(subComponent?.tags && subComponent.tags.length > 0)}
            >
              {subComponent.long_description}
            </SubComponentLongDescription>
          )}
          {subComponent?.documentation_url && (
            <DocumentationButtonContainer>
              <Button
                variant="outlined"
                component="a"
                startIcon={<OpenInNew />}
                href={subComponent.documentation_url}
                target="_blank"
                rel="noopener noreferrer"
              >
                View Documentation
              </Button>
            </DocumentationButtonContainer>
          )}
        </StyledPaper>
      )}

      {subComponentStatus?.suspected_outage && (
        <SuspectedReportsBanner
          suspected={subComponentStatus.suspected_outage}
          componentSlug={componentSlug || ''}
          subComponentName={subComponentName}
          onReportClick={() => setReportDialogOpen(true)}
        />
      )}

      <StyledPaper>
        {loading && (
          <LoadingBox>
            <CircularProgress />
          </LoadingBox>
        )}

        {(validationError || error) && (
          <ErrorAlert severity="error">{validationError || error}</ErrorAlert>
        )}

        {!loading && !validationError && !error && (
          <Box>
            <OutageFilterToolbar data-tour="subcomponent-detail-filter">
              <TextField
                type="date"
                label="From"
                size="small"
                value={dateStart || ''}
                onChange={(e) => handleDateChange('start', e.target.value)}
                slotProps={{ inputLabel: { shrink: true } }}
                sx={{ width: 160 }}
              />
              <TextField
                type="date"
                label="To"
                size="small"
                value={dateEnd || ''}
                onChange={(e) => handleDateChange('end', e.target.value)}
                slotProps={{ inputLabel: { shrink: true } }}
                sx={{ width: 160 }}
              />
              {(dateStart || dateEnd) && (
                <Tooltip title="Clear date filter">
                  <IconButton size="small" onClick={clearDateFilter}>
                    <Clear fontSize="small" />
                  </IconButton>
                </Tooltip>
              )}
              <ToggleButtonGroup
                value={statusFilter}
                exclusive
                onChange={handleFilterChange}
                aria-label="outage status filter"
                size="small"
                sx={{ marginLeft: 'auto' }}
              >
                <ToggleButton value="all" aria-label="all outages">
                  All
                </ToggleButton>
                <ToggleButton value="ongoing" aria-label="ongoing outages">
                  Ongoing
                </ToggleButton>
                <ToggleButton value="resolved" aria-label="resolved outages">
                  Resolved
                </ToggleButton>
              </ToggleButtonGroup>
            </OutageFilterToolbar>
            <DataGridContainer data-tour="subcomponent-detail-grid">
              <StyledDataGrid
                rows={sortedOutages}
                columns={columns}
                pageSizeOptions={[10, 25, 50, 100]}
                initialState={{
                  pagination: {
                    paginationModel: { pageSize: 25 },
                  },
                }}
                disableRowSelectionOnClick
                getRowId={(row) => row.ID}
              />
            </DataGridContainer>
          </Box>
        )}
      </StyledPaper>

      <UpsertOutageModal
        open={createOutageModalOpen}
        onClose={() => setCreateOutageModalOpen(false)}
        onSuccess={handleOutageAction}
        componentName={componentName || ''}
        subComponentName={subComponentName || ''}
      />

      <Dialog
        open={reportDialogOpen}
        onClose={() => {
          setReportDialogOpen(false)
          setReportError(null)
        }}
      >
        <DialogTitle>Report Issue</DialogTitle>
        <DialogContent>
          {reportError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {reportError}
            </Alert>
          )}
          <DialogContentText>
            Report a suspected issue with {componentName} / {subComponentName}. If others have
            already reported this, your report will be added to the existing one.
          </DialogContentText>
          <TextField
            autoFocus
            margin="dense"
            label="What are you experiencing? (optional)"
            fullWidth
            multiline
            minRows={2}
            maxRows={4}
            value={reportDescription}
            onChange={(e) => setReportDescription(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setReportDialogOpen(false)
              setReportError(null)
            }}
          >
            Cancel
          </Button>
          <Button onClick={handleReportSubmit} variant="contained" disabled={reportSubmitting}>
            {reportSubmitting ? 'Submitting...' : 'Submit'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={reportSuccess !== null}
        autoHideDuration={4000}
        onClose={() => setReportSuccess(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert severity="success" onClose={() => setReportSuccess(null)} sx={{ width: '100%' }}>
          {reportSuccess}
        </Alert>
      </Snackbar>
    </PageContainer>
  )
}

export default SubComponentDetails
