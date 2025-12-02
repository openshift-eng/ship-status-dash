import { Stop } from '@mui/icons-material'
import { Button, Tooltip, TextField, Box } from '@mui/material'
import { useState } from 'react'

import type { Outage } from '../../../types'
import { modifyOutageEndpoint } from '../../../utils/endpoints'
import { getCurrentLocalTime } from '../../../utils/helpers'

import ConfirmationModal from './ConfirmationModal'

interface EndOutageProps {
  outage: Outage
  onEndSuccess: () => void
  onError: (error: string) => void
}

const EndOutage = ({ outage, onEndSuccess, onError }: EndOutageProps) => {
  const [endDialogOpen, setEndDialogOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [endTime, setEndTime] = useState(getCurrentLocalTime())

  const handleEndClick = () => {
    setEndTime(getCurrentLocalTime())
    setEndDialogOpen(true)
  }

  const handleEndConfirm = () => {
    setIsLoading(true)
    fetch(modifyOutageEndpoint(outage.component_name, outage.sub_component_name, outage.ID), {
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
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to resolve outage: ${response.statusText}`)
        }
        onEndSuccess()
      })
      .catch((err) => {
        onError(err instanceof Error ? err.message : 'Failed to resolve outage')
      })
      .finally(() => {
        setIsLoading(false)
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
      <ConfirmationModal
        open={endDialogOpen}
        onClose={handleEndCancel}
        onConfirm={handleEndConfirm}
        title="Resolve"
        description="Configure the end time for this outage:"
        confirmButtonText="Resolve"
        confirmButtonColor="primary"
        isLoading={isLoading}
        outage={outage}
      >
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
      </ConfirmationModal>
    </>
  )
}

export default EndOutage
