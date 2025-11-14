import ClearIcon from '@mui/icons-material/Clear'
import {
  Alert,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  IconButton,
  InputAdornment,
  InputLabel,
  MenuItem,
  Select,
  styled,
  TextField,
  Typography,
} from '@mui/material'
import type { ChangeEvent } from 'react'
import { useEffect, useState } from 'react'

import type { Outage } from '../../../types'
import { createOutageEndpoint, modifyOutageEndpoint } from '../../../utils/endpoints'
import { formatDateForDateTimeLocal, getCurrentLocalTime } from '../../../utils/helpers'

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

interface UpsertOutageModalProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
  componentName: string
  subComponentName: string
  outage?: Outage
}

interface OutageFormData {
  severity: string
  description: string
  triage_notes: string
  start_time: string
  end_time: string
  confirmed: boolean
}

const UpsertOutageModal = ({
  open,
  onClose,
  onSuccess,
  componentName,
  subComponentName,
  outage,
}: UpsertOutageModalProps) => {
  const isUpdateMode = !!outage

  const [formData, setFormData] = useState<OutageFormData>({
    severity: outage?.severity || '',
    description: outage?.description || '',
    triage_notes: outage?.triage_notes || '',
    start_time: outage
      ? formatDateForDateTimeLocal(new Date(outage.start_time))
      : getCurrentLocalTime(),
    end_time: outage?.end_time.Valid
      ? formatDateForDateTimeLocal(new Date(outage.end_time.Time))
      : '',
    confirmed: outage?.confirmed_at.Valid || false,
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Update form data when outage prop changes
  useEffect(() => {
    if (outage) {
      setFormData({
        severity: outage.severity,
        description: outage.description || '',
        triage_notes: outage.triage_notes || '',
        start_time: formatDateForDateTimeLocal(new Date(outage.start_time)),
        end_time: outage.end_time.Valid
          ? formatDateForDateTimeLocal(new Date(outage.end_time.Time))
          : '',
        confirmed: outage.confirmed_at.Valid,
      })
    } else {
      // Reset to default values for create mode
      setFormData({
        severity: '',
        description: '',
        triage_notes: '',
        start_time: getCurrentLocalTime(),
        end_time: '',
        confirmed: true,
      })
    }
  }, [outage])

  const handleInputChange =
    (field: keyof OutageFormData) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setFormData((prev) => ({
        ...prev,
        [field]: event.target.value,
      }))
      // Clear error when user starts typing
      if (error) {
        setError(null)
      }
    }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const handleSelectChange = (field: keyof OutageFormData) => (event: any) => {
    setFormData((prev) => ({
      ...prev,
      [field]: event.target.value,
    }))
    // Clear error when user starts typing
    if (error) {
      setError(null)
    }
  }

  const handleCheckboxChange =
    (field: keyof OutageFormData) => (event: ChangeEvent<HTMLInputElement>) => {
      setFormData((prev) => ({
        ...prev,
        [field]: event.target.checked,
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
    if (!formData.start_time) {
      return 'Start time is required'
    }
    if (formData.end_time && formData.start_time) {
      const startTime = new Date(formData.start_time)
      const endTime = new Date(formData.end_time)
      if (endTime <= startTime) {
        return 'End time must be after start time'
      }
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

    const requestData: Record<string, unknown> = {
      severity: formData.severity,
      description: formData.description || undefined,
      start_time: new Date(formData.start_time).toISOString(),
      triage_notes: formData.triage_notes || undefined,
      confirmed: formData.confirmed,
    }

    // Handle end_time - either set it or clear it
    if (formData.end_time) {
      requestData.end_time = {
        Time: new Date(formData.end_time).toISOString(),
        Valid: true,
      }
    } else if (isUpdateMode && outage?.end_time.Valid) {
      // If we're updating and the outage previously had an end_time, set Valid to false to mark it as ongoing again
      requestData.end_time = {
        Time: new Date().toISOString(),
        Valid: false,
      }
    }

    if (!isUpdateMode) {
      requestData.discovered_from = 'frontend'
      // Don't require confirmation for new outages added from the frontend
      requestData.confirmed = true
    }

    const url = isUpdateMode
      ? modifyOutageEndpoint(componentName, subComponentName, outage?.id)
      : createOutageEndpoint(componentName, subComponentName)

    const method = isUpdateMode ? 'PATCH' : 'POST'

    fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(requestData),
      credentials: 'include',
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
        // Only reset form in create mode, update mode closes the modal
        if (!isUpdateMode) {
          setFormData({
            severity: '',
            description: '',
            triage_notes: '',
            start_time: getCurrentLocalTime(),
            end_time: '',
            confirmed: true,
          })
        }
        onSuccess()
        onClose()
      })
      .catch((err) => {
        setError(
          err instanceof Error
            ? err.message
            : `Failed to ${isUpdateMode ? 'update' : 'create'} outage`,
        )
      })
      .finally(() => {
        setLoading(false)
      })
  }

  const handleClose = () => {
    if (!loading) {
      // Only reset form in create mode, update mode preserves the data
      if (!isUpdateMode) {
        setFormData({
          severity: '',
          description: '',
          triage_notes: '',
          start_time: getCurrentLocalTime(),
          end_time: '',
          confirmed: false,
        })
      }
      setError(null)
      onClose()
    }
  }

  return (
    <StyledDialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Typography variant="h6">
          {isUpdateMode ? 'Update Outage' : 'Report Outage'} - {componentName} / {subComponentName}
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
          slotProps={{
            inputLabel: {
              shrink: true,
            },
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
          label="Triage Notes"
          multiline
          rows={2}
          value={formData.triage_notes}
          onChange={handleInputChange('triage_notes')}
          placeholder="Initial triage notes..."
        />

        <StyledTextField
          fullWidth
          label="End Time"
          type="datetime-local"
          value={formData.end_time}
          onChange={handleInputChange('end_time')}
          slotProps={{
            inputLabel: {
              shrink: true,
            },
            input: {
              endAdornment: formData.end_time ? (
                <InputAdornment position="end">
                  <IconButton
                    onClick={() => {
                      setFormData((prev) => ({ ...prev, end_time: '' }))
                      setError(null)
                    }}
                    edge="end"
                    size="small"
                    aria-label="clear end time"
                  >
                    <ClearIcon />
                  </IconButton>
                </InputAdornment>
              ) : undefined,
            },
          }}
          helperText={
            isUpdateMode && outage?.end_time.Valid
              ? 'Clear this field to mark outage as ongoing again'
              : 'Leave empty if outage is still ongoing'
          }
        />

        {isUpdateMode && (
          <FormControlLabel
            control={
              <Checkbox
                checked={formData.confirmed}
                onChange={handleCheckboxChange('confirmed')}
                color="primary"
                disabled={!isUpdateMode}
              />
            }
            label="Confirmed"
          />
        )}
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
          {loading
            ? isUpdateMode
              ? 'Updating...'
              : 'Reporting...'
            : isUpdateMode
              ? 'Update Outage'
              : 'Report Outage'}
        </Button>
      </DialogActions>
    </StyledDialog>
  )
}

export default UpsertOutageModal
