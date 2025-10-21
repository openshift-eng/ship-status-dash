import { Delete } from '@mui/icons-material'
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
  styled,
} from '@mui/material'
import React, { useState } from 'react'

import type { Outage } from '../types'
import { modifyOutageEndpoint } from '../utils/endpoints'

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

interface DeleteOutageProps {
  outage: Outage
  onDeleteSuccess: () => void
  onError: (error: string) => void
}

const DeleteOutage: React.FC<DeleteOutageProps> = ({ outage, onDeleteSuccess, onError }) => {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [snackbarOpen, setSnackbarOpen] = useState(false)
  const [snackbarMessage, setSnackbarMessage] = useState('')

  const handleDeleteClick = () => {
    setDeleteDialogOpen(true)
  }

  const handleDeleteConfirm = () => {
    fetch(modifyOutageEndpoint(outage.component_name, outage.sub_component_name, outage.id), {
      method: 'DELETE',
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to delete outage: ${response.statusText}`)
        }
        setSnackbarMessage('Outage deleted successfully')
        setSnackbarOpen(true)
        onDeleteSuccess()
      })
      .catch((err) => {
        onError(err instanceof Error ? err.message : 'Failed to delete outage')
      })
      .finally(() => {
        setDeleteDialogOpen(false)
      })
  }

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false)
  }

  return (
    <>
      <Tooltip title="Delete outage" arrow>
        <Button size="small" color="error" onClick={handleDeleteClick} startIcon={<Delete />}>
          Delete
        </Button>
      </Tooltip>
      <StyledDialog open={deleteDialogOpen} onClose={handleDeleteCancel} maxWidth="sm" fullWidth>
        <DialogTitle>Delete Outage</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete this outage? This action cannot be undone.
          </Typography>
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
          <Button onClick={handleDeleteCancel}>Cancel</Button>
          <Button onClick={handleDeleteConfirm} color="error" variant="contained">
            Delete
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

export default DeleteOutage
