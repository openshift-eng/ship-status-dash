import {
  Box,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Alert,
  styled,
  CircularProgress,
} from '@mui/material'
import React, { useState } from 'react'

import { outageEndpoint } from '../utils/endpoints'
import { getCurrentLocalTime } from '../utils/helpers'

const StyledDialog = styled(Dialog)(({ theme }) => ({
  '& .MuiDialog-paper': {
    borderRadius: theme.spacing(2),
  },
}))

const StyledTextField = styled(TextField)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const StyledFormControl = styled(FormControl)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

interface CreateOutageModalProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
  componentName: string
  subComponentName: string
}

interface OutageFormData {
  severity: string
  description: string
  created_by: string
  triage_notes: string
  start_time: string
}

const CreateOutageModal: React.FC<CreateOutageModalProps> = ({
  open,
  onClose,
  onSuccess,
  componentName,
  subComponentName,
}) => {
  const [formData, setFormData] = useState<OutageFormData>({
    severity: '',
    description: '',
    created_by: '',
    triage_notes: '',
    start_time: getCurrentLocalTime(),
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleInputChange = (field: keyof OutageFormData) => (
    event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    setFormData(prev => ({
      ...prev,
      [field]: event.target.value,
    }))
    // Clear error when user starts typing
    if (error) {
      setError(null)
    }
  }

  const handleSelectChange = (field: keyof OutageFormData) => (
    event: any
  ) => {
    setFormData(prev => ({
      ...prev,
      [field]: event.target.value,
    }))
    // Clear error when user starts typing
    if (error) {
      setError(null)
    }
  }

  const validateForm = (): string | null => {
    if (!formData.severity) {
      return 'Severity is required'
    }
    if (!formData.created_by.trim()) {
      return 'Created by is required'
    }
    if (!formData.start_time) {
      return 'Start time is required'
    }
    return null
  }

  const handleSubmit = () => {
    const validationError = validateForm()
    if (validationError) {
      setError(validationError)
      return
    }

    setLoading(true)
    setError(null)

    fetch(outageEndpoint(componentName, subComponentName), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        severity: formData.severity,
        description: formData.description || undefined,
        discovered_from: 'frontend',
        created_by: formData.created_by,
        triage_notes: formData.triage_notes || undefined,
        start_time: new Date(formData.start_time).toISOString(),
      }),
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((errorData) => {
            throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`)
          })
        }
        return response.json()
      })
      .then(() => {
        // Reset form and close modal
        setFormData({
          severity: '',
          description: '',
          created_by: '',
          triage_notes: '',
          start_time: getCurrentLocalTime(),
        })
        onSuccess()
        onClose()
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to create outage')
      })
      .finally(() => {
        setLoading(false)
      })
  }

  const handleClose = () => {
    if (!loading) {
      setFormData({
        severity: '',
        description: '',
        created_by: '',
        triage_notes: '',
        start_time: getCurrentLocalTime(),
      })
      setError(null)
      onClose()
    }
  }

  return (
    <StyledDialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Typography variant="h6">
          Report Outage - {componentName} / {subComponentName}
        </Typography>
      </DialogTitle>
      <DialogContent>
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        <StyledFormControl fullWidth required>
          <InputLabel>Severity</InputLabel>
          <Select
            value={formData.severity}
            onChange={handleSelectChange('severity')}
            label="Severity"
          >
            <MenuItem value="Down">Down</MenuItem>
            <MenuItem value="Degraded">Degraded</MenuItem>
            <MenuItem value="Suspected">Suspected</MenuItem>
          </Select>
        </StyledFormControl>

        <StyledTextField
          fullWidth
          required
          label="Start Time"
          type="datetime-local"
          value={formData.start_time}
          onChange={handleInputChange('start_time')}
          InputLabelProps={{
            shrink: true,
          }}
        />

        <StyledTextField
          fullWidth
          label="Description"
          multiline
          rows={3}
          value={formData.description}
          onChange={handleInputChange('description')}
          placeholder="Describe the outage..."
        />

        <StyledTextField
          fullWidth
          required
          label="Created By"
          value={formData.created_by}
          onChange={handleInputChange('created_by')}
          placeholder="Your name or identifier"
        />

        <StyledTextField
          fullWidth
          label="Triage Notes"
          multiline
          rows={2}
          value={formData.triage_notes}
          onChange={handleInputChange('triage_notes')}
          placeholder="Initial triage notes..."
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          onClick={handleSubmit}
          variant="contained"
          disabled={loading}
          startIcon={loading ? <CircularProgress size={20} /> : null}
        >
          {loading ? 'Reporting...' : 'Report Outage'}
        </Button>
      </DialogActions>
    </StyledDialog>
  )
}

export default CreateOutageModal
