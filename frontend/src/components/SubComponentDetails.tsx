import { CheckCircle, Error, ReportProblem } from '@mui/icons-material'
import {
  Box,
  Typography,
  Button,
  styled,
  CircularProgress,
  Alert,
  Container,
  Paper,
  Tooltip,
} from '@mui/material'
import { DataGrid } from '@mui/x-data-grid'
import type { GridColDef } from '@mui/x-data-grid'
import React, { useState, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import OutageActions from './OutageActions'
import OutageDetailsButton from './OutageDetailsButton'
import { SeverityChip } from './StatusColors'
import UpsertOutageModal from './UpsertOutageModal'
import type { Outage } from '../types'
import { createOutageEndpoint } from '../utils/endpoints'
import { relativeTime, getStatusBackgroundColor } from '../utils/helpers'

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
  backgroundColor: 'white',
  color: theme.palette.text.primary,
  '&:hover': {
    backgroundColor: theme.palette.grey[100],
  },
}))

const StyledDataGrid = styled(DataGrid)(() => ({
  '& .MuiDataGrid-cell': {
    borderBottom: '1px solid #e0e0e0',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  '& .MuiDataGrid-row:hover': {
    backgroundColor: '#f5f5f5',
  },
}))

const SubComponentDetails: React.FC = () => {
  const navigate = useNavigate()
  const { componentName, subComponentName } = useParams<{
    componentName: string
    subComponentName: string
  }>()
  const [outages, setOutages] = useState<Outage[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [createOutageModalOpen, setCreateOutageModalOpen] = useState(false)

  const fetchOutages = () => {
    if (!componentName || !subComponentName) {
      setError('Missing component or subcomponent name')
      return
    }

    setLoading(true)
    setError(null)

    fetch(createOutageEndpoint(componentName, subComponentName))
      .then((response) => {
        if (!response.ok) {
          setError(`Failed to fetch outages: ${response.statusText}`)
          return
        }
        return response.json()
      })
      .then((data) => {
        if (data) {
          setOutages(data)
        }
      })
      .catch(() => {
        setError('Failed to fetch outages')
      })
      .finally(() => {
        setLoading(false)
      })
  }

  useEffect(() => {
    fetchOutages()
  }, [componentName, subComponentName])

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  const getStatusText = (outage: Outage) => {
    if (outage.end_time.Valid) {
      return 'Resolved'
    }
    return 'Active'
  }

  const getSubComponentStatus = () => {
    const activeOutages = outages.filter((outage) => !outage.end_time.Valid)
    if (activeOutages.length === 0) {
      return 'Healthy'
    }

    // Check for highest severity among active outages
    const hasDownOutage = activeOutages.some((outage) => outage.severity === 'Down')
    if (hasDownOutage) {
      return 'Down'
    }

    const hasDegradedOutage = activeOutages.some((outage) => outage.severity === 'Degraded')
    if (hasDegradedOutage) {
      return 'Degraded'
    }

    return 'Suspected'
  }

  const handleOutageAction = () => {
    fetchOutages()
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
    {
      field: 'actions',
      headerName: 'Actions',
      width: 100,
      sortable: false,
      filterable: false,
      renderCell: (params) => {
        const outage = params.row as Outage
        return <OutageActions outage={outage} onSuccess={handleOutageAction} onError={setError} />
      },
    },
  ]

  // Sort outages: active first, then by start time descending
  const sortedOutages = [...outages].sort((a, b) => {
    const aActive = !a.end_time.Valid
    const bActive = !b.end_time.Valid

    if (aActive && !bActive) return -1
    if (!aActive && bActive) return 1

    return new Date(b.start_time).getTime() - new Date(a.start_time).getTime()
  })

  if (!componentName || !subComponentName) {
    return (
      <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
        <Alert severity="error">Invalid component or subcomponent</Alert>
      </Container>
    )
  }

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
      <StyledPaper status={getSubComponentStatus()}>
        <HeaderBox status={getSubComponentStatus()}>
          <Typography variant="h4">
            {componentName} / {subComponentName} - Outage History
          </Typography>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button
              variant="contained"
              color="error"
              startIcon={<ReportProblem />}
              onClick={() => setCreateOutageModalOpen(true)}
            >
              Report Outage
            </Button>
            <StyledButton variant="contained" onClick={() => navigate(`/${componentName}`)}>
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
              getRowId={(row) => row.id}
            />
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
