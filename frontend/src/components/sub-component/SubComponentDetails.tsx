import { CheckCircle, Error, ReportProblem, Warning } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  styled,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import { DataGrid } from '@mui/x-data-grid'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { useAuth } from '../../contexts/AuthContext'
import type { ComponentStatus, Outage } from '../../types'
import {
  getComponentInfoEndpoint,
  getSubComponentOutagesEndpoint,
  getSubComponentStatusEndpoint,
} from '../../utils/endpoints'
import { getStatusBackgroundColor, relativeTime } from '../../utils/helpers'
import { deslugify } from '../../utils/slugify'
import OutageActions from '../outage/actions/OutageActions'
import UpsertOutageModal from '../outage/actions/UpsertOutageModal'
import OutageDetailsButton from '../outage/OutageDetailsButton'
import { SeverityChip } from '../StatusColors'

const HeaderBox = styled(Box)<{ status: string }>(({ theme, status }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 24,
  backgroundColor: getStatusBackgroundColor(theme, status),
  padding: theme.spacing(2),
  borderRadius: theme.spacing(1),
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
  backgroundColor: status ? getStatusBackgroundColor(theme, status) : undefined,
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
  '& .MuiDataGrid-cell': {
    borderBottom: `1px solid ${theme.palette.divider}`,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  '& .MuiDataGrid-row:hover': {
    backgroundColor: theme.palette.action.hover,
  },
}))

const SubComponentDetails = () => {
  const navigate = useNavigate()
  const { componentSlug, subComponentSlug } = useParams<{
    componentSlug: string
    subComponentSlug: string
  }>()
  const { isComponentAdmin } = useAuth()
  const [outages, setOutages] = useState<Outage[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [createOutageModalOpen, setCreateOutageModalOpen] = useState(false)
  const [subComponentStatus, setSubComponentStatus] = useState<ComponentStatus | null>(null)
  const [subComponentRequiresConfirmation, setSubComponentRequiresConfirmation] =
    useState<boolean>(false)
  const [statusFilter, setStatusFilter] = useState<'all' | 'ongoing' | 'resolved'>('all')

  const componentName = componentSlug ? deslugify(componentSlug) : ''
  const subComponentName = subComponentSlug ? deslugify(subComponentSlug) : ''
  const isAdmin = isComponentAdmin(componentSlug || '')

  const fetchData = useCallback(() => {
    if (!componentName || !subComponentName) {
      setError('Missing component or subcomponent name')
      return
    }

    setLoading(true)
    setError(null)

    // Fetch outages, status, and component configuration in parallel
    Promise.all([
      fetch(getSubComponentOutagesEndpoint(componentName, subComponentName)),
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
            // Set default filter to 'ongoing' if there are any ongoing outages
            const hasOngoing = outagesData.some((outage: Outage) => !outage.end_time.Valid)
            if (hasOngoing) {
              setStatusFilter('ongoing')
            } else {
              setStatusFilter('all')
            }
          }
          if (statusData) {
            setSubComponentStatus(statusData)
          }
          if (componentData) {
            // Set the confirmation requirement based on the subcomponent configuration
            const subComponent = componentData.sub_components.find(
              (sub: { name: string; requires_confirmation: boolean }) =>
                sub.name === subComponentName,
            )
            setSubComponentRequiresConfirmation(subComponent?.requires_confirmation || false)
          }
        }
      })
      .catch(() => {
        setError('Failed to fetch data')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponentName])

  useEffect(() => {
    fetchData()
  }, [fetchData])

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
            {isActive ? <Error color="error" /> : <CheckCircle color="success" />}
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
          label={params.value}
          severity={params.value}
          size="small"
          variant="outlined"
        />
      ),
    },
    ...(subComponentRequiresConfirmation
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
                  {isConfirmed ? <CheckCircle color="success" /> : <Warning color="warning" />}
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
        return (
          <Typography variant="body2" color="error">
            Ongoing
          </Typography>
        )
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

  // Filter outages based on selected filter
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

  if (!componentName || !subComponentName) {
    return (
      <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
        <Alert severity="error">Invalid component or subcomponent</Alert>
      </Container>
    )
  }

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <StyledPaper status={subComponentStatus?.status || 'Unknown'}>
        <HeaderBox status={subComponentStatus?.status || 'Unknown'}>
          <Box>
            <Typography variant="h4">
              {componentName} / {subComponentName} - Outages
            </Typography>
            {subComponentStatus?.last_ping_time && (
              <Typography variant="body2" sx={{ mt: 1, opacity: 0.8 }}>
                Last ping: {relativeTime(new Date(subComponentStatus.last_ping_time), new Date())}
              </Typography>
            )}
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            {isAdmin && (
              <Button
                variant="contained"
                color="error"
                startIcon={<ReportProblem />}
                onClick={() => setCreateOutageModalOpen(true)}
              >
                Report Outage
              </Button>
            )}
            <StyledButton variant="contained" onClick={() => navigate(`/${componentSlug}`)}>
              {componentName} Details
            </StyledButton>
          </Box>
        </HeaderBox>
      </StyledPaper>

      <StyledPaper>
        {loading && (
          <LoadingBox>
            <CircularProgress />
          </LoadingBox>
        )}

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        {!loading && !error && (
          <Box>
            <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}>
              <ToggleButtonGroup
                value={statusFilter}
                exclusive
                onChange={handleFilterChange}
                aria-label="outage status filter"
                size="small"
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
            </Box>
            <Box sx={{ height: 600, width: '100%' }}>
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
            </Box>
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
    </Container>
  )
}

export default SubComponentDetails
