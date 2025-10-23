import {
    Box,
    Button,
    Dialog,
    DialogActions,
    DialogContent,
    DialogTitle,
    styled,
    Typography,
} from '@mui/material'
import type { ReactNode } from 'react'
import React from 'react'

const StyledDialog = styled(Dialog)(({ theme }) => ({
  '& .MuiDialog-paper': {
    borderRadius: theme.spacing(2),
  },
}))

const OutageDetailsBox = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(2),
  padding: theme.spacing(2),
  backgroundColor:
    theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[100],
  borderRadius: theme.spacing(1),
  border: `1px solid ${theme.palette.divider}`,
}))

interface ConfirmationModalProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: string
  confirmButtonText: string
  confirmButtonColor?: 'primary' | 'error' | 'secondary'
  isLoading: boolean
  children?: ReactNode
  outage: {
    severity: string
    description?: string
    start_time: string
  }
}

const ConfirmationModal: React.FC<ConfirmationModalProps> = ({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmButtonText,
  confirmButtonColor = 'primary',
  isLoading,
  children,
  outage,
}) => {
  const handleConfirm = () => {
    onConfirm()
  }

  const handleClose = () => {
    onClose()
  }

  return (
    <StyledDialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Typography>{description}</Typography>
        {children}
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
        <Button onClick={handleClose} disabled={isLoading}>
          Cancel
        </Button>
        <Button
          onClick={handleConfirm}
          color={confirmButtonColor}
          variant="contained"
          disabled={isLoading}
        >
          {confirmButtonText}
        </Button>
      </DialogActions>
    </StyledDialog>
  )
}

export default ConfirmationModal
