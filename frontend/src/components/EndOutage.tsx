import { Stop } from '@mui/icons-material'
import {
  Button,
  Tooltip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Typography,
  Box,
  Snackbar,
  TextField,
  styled,
} from '@mui/material'
import React, { useState } from 'react'

import { modifyOutageEndpoint } from '../utils/endpoints'
import { getCurrentLocalTime } from '../utils/helpers'
import type { Outage } from '../types'

const StyledDialog = styled(Dialog)(({ theme }) => ({
  '& .MuiDialog-paper': {
    borderRadius: theme.spacing(2),
  },
}))

const OutageDetailsBox = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(2),
  padding: theme.spacing(2),
  backgroundColor: theme.palette.grey[100],
  borderRadius: theme.spacing(1),
}))

interface EndOutageProps {
  outage: Outage
  onEndSuccess: () => void
  onError: (error: string) => void
}

const EndOutage: React.FC<EndOutageProps> = ({ outage, onEndSuccess, onError }) => {
  const [endDialogOpen, setEndDialogOpen] = useState(false)
  const [snackbarOpen, setSnackbarOpen] = useState(false)
  const [snackbarMessage, setSnackbarMessage] = useState('')
  const [endTime, setEndTime] = useState(getCurrentLocalTime())

  const handleEndClick = () => {
    setEndTime(getCurrentLocalTime())
    setEndDialogOpen(true)
  }

  const handleEndConfirm = () => {
    fetch(modifyOutageEndpoint(outage.component_name, outage.sub_component_name, outage.id), {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        end_time: {
          Time: new Date(endTime).toISOString(),
          Valid: true,
        },
      }),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to resolve outage: ${response.statusText}`)
        }
        setSnackbarMessage('Outage resolved successfully')
        setSnackbarOpen(true)
        onEndSuccess()
      })
      .catch((err) => {
        onError(err instanceof Error ? err.message : 'Failed to resolve outage')
      })
      .finally(() => {
        setEndDialogOpen(false)
      })
  }

  const handleEndCancel = () => {
    setEndDialogOpen(false)
  }

  return (
    <>
      <Tooltip title="Resolve outage" arrow>
        <Button size="small" color="primary" onClick={handleEndClick} startIcon={<Stop />}>
          Resolve
        </Button>
      </Tooltip>
      <StyledDialog open={endDialogOpen} onClose={handleEndCancel} maxWidth="sm" fullWidth>
        <DialogTitle>Resolve</DialogTitle>
        <DialogContent>
          <Typography>
            Configure the end time for this outage:
          </Typography>
          <Box sx={{ mt: 2 }}>
            <TextField
              label="End Time"
              type="datetime-local"
              value={endTime}
              onChange={(e) => setEndTime(e.target.value)}
              fullWidth
              InputLabelProps={{
                shrink: true,
              }}
            />
          </Box>
          <OutageDetailsBox>
            <Typography variant="body2">
              <strong>Severity:</strong> {outage.severity}
            </Typography>
            <Typography variant="body2">
              <strong>Description:</strong> {outage.description || 'No description'}
            </Typography>
            <Typography variant="body2">
              <strong>Start Time:</strong> {new Date(outage.start_time).toLocaleString()}
            </Typography>
          </OutageDetailsBox>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleEndCancel}>Cancel</Button>
          <Button onClick={handleEndConfirm} color="primary" variant="contained">
            Resolve
          </Button>
        </DialogActions>
      </StyledDialog>

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={4000}
        onClose={() => setSnackbarOpen(false)}
        message={snackbarMessage}
      />
    </>
  )
}

export default EndOutage
