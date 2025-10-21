import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Box,
  Chip,
  Divider,
} from '@mui/material'
import React from 'react'

import type { Outage } from '../types'
import { relativeTime, relativeDuration } from '../utils/helpers'

interface OutageDetailsModalProps {
  open: boolean
  onClose: () => void
  outage: Outage
}

const OutageDetailsModal: React.FC<OutageDetailsModalProps> = ({ open, onClose, outage }) => {
  const getSeverityColor = (severity: string) => {
    switch (severity.toLowerCase()) {
      case 'down':
        return 'error'
      case 'degraded':
        return 'warning'
      case 'suspected':
        return 'info'
      default:
        return 'default'
    }
  }

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

  const formatDuration = (startTime: string, endTime?: { Time: string; Valid: boolean }) => {
    const start = new Date(startTime)
    const end = endTime?.Valid ? new Date(endTime.Time) : new Date()
    const durationSeconds = Math.floor((end.getTime() - start.getTime()) / 1000)
    const duration = relativeDuration(durationSeconds)
    return `${Math.round(Number(duration.value))} ${duration.units}`
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box display="flex" alignItems="center" gap={2}>
          <Typography variant="h6">Outage Details</Typography>
          <Chip label={outage.severity} color={getSeverityColor(outage.severity)} size="small" />
        </Box>
      </DialogTitle>

      <DialogContent>
        <Box display="flex" flexDirection="column" gap={3}>
          {/* Basic Information */}
          <Box>
            <Typography variant="h6" gutterBottom>
              Basic Information
            </Typography>
            <Box display="flex" flexDirection="column" gap={1}>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Component
                </Typography>
                <Typography variant="body1">{outage.component_name}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Sub-Component
                </Typography>
                <Typography variant="body1">{outage.sub_component_name}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Severity
                </Typography>
                <Chip
                  label={outage.severity}
                  color={getSeverityColor(outage.severity)}
                  size="small"
                />
              </Box>
              {outage.description && (
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    Description
                  </Typography>
                  <Typography variant="body1">{outage.description}</Typography>
                </Box>
              )}
            </Box>
          </Box>

          <Divider />

          {/* Timing Information */}
          <Box>
            <Typography variant="h6" gutterBottom>
              Timing Information
            </Typography>
            <Box display="flex" flexDirection="column" gap={1}>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Start Time
                </Typography>
                <Typography variant="body1">{formatDateTime(outage.start_time)}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  End Time
                </Typography>
                <Typography variant="body1">{formatNullableDateTime(outage.end_time)}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Duration
                </Typography>
                <Typography variant="body1">
                  {formatDuration(outage.start_time, outage.end_time)}
                </Typography>
              </Box>
            </Box>
          </Box>

          <Divider />

          {/* User Information */}
          <Box>
            <Typography variant="h6" gutterBottom>
              User Information
            </Typography>
            <Box display="flex" flexDirection="column" gap={1}>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Created By
                </Typography>
                <Typography variant="body1">{outage.created_by}</Typography>
              </Box>
              {outage.resolved_by && (
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    Resolved By
                  </Typography>
                  <Typography variant="body1">{outage.resolved_by}</Typography>
                </Box>
              )}
              {outage.confirmed_by && (
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    Confirmed By
                  </Typography>
                  <Typography variant="body1">{outage.confirmed_by}</Typography>
                </Box>
              )}
            </Box>
          </Box>

          <Divider />

          {/* Additional Information */}
          <Box>
            <Typography variant="h6" gutterBottom>
              Additional Information
            </Typography>
            <Box display="flex" flexDirection="column" gap={1}>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Discovered From
                </Typography>
                <Typography variant="body1">{outage.discovered_from}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Confirmed
                </Typography>
                <Chip
                  label={outage.confirmed_at.Valid ? 'Yes' : 'No'}
                  color={outage.confirmed_at.Valid ? 'success' : 'default'}
                  size="small"
                />
                {outage.confirmed_at.Valid && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                    Confirmed at: {formatDateTime(outage.confirmed_at.Time)}
                  </Typography>
                )}
              </Box>
              {outage.triage_notes && (
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    Triage Notes
                  </Typography>
                  <Typography variant="body1">{outage.triage_notes}</Typography>
                </Box>
              )}
            </Box>
          </Box>

          <Divider />

          {/* System Information */}
          <Box>
            <Typography variant="h6" gutterBottom>
              System Information
            </Typography>
            <Box display="flex" flexDirection="column" gap={1}>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Outage ID
                </Typography>
                <Typography variant="body1">{outage.id}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Created At
                </Typography>
                <Typography variant="body1">{formatDateTime(outage.created_at)}</Typography>
              </Box>
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Updated At
                </Typography>
                <Typography variant="body1">{formatDateTime(outage.updated_at)}</Typography>
              </Box>
            </Box>
          </Box>
        </Box>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default OutageDetailsModal
