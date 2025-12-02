import { CheckCircle } from '@mui/icons-material'
import { Button, Tooltip } from '@mui/material'
import { useState } from 'react'

import type { Outage } from '../../../types'
import { modifyOutageEndpoint } from '../../../utils/endpoints'

interface ConfirmOutageProps {
  outage: Outage
  onConfirmSuccess: () => void
  onError: (error: string) => void
}

const ConfirmOutage = ({ outage, onConfirmSuccess, onError }: ConfirmOutageProps) => {
  const [isLoading, setIsLoading] = useState(false)

  const handleConfirmClick = () => {
    setIsLoading(true)

    fetch(modifyOutageEndpoint(outage.component_name, outage.sub_component_name, outage.ID), {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        confirmed: true,
      }),
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to confirm outage: ${response.statusText}`)
        }
        onConfirmSuccess()
      })
      .catch((err) => {
        onError(err instanceof Error ? err.message : 'Failed to confirm outage')
      })
      .finally(() => {
        setIsLoading(false)
      })
  }

  return (
    <Tooltip title="Confirm outage" arrow>
      <Button
        size="small"
        color="primary"
        onClick={handleConfirmClick}
        startIcon={<CheckCircle />}
        disabled={isLoading}
      >
        Confirm
      </Button>
    </Tooltip>
  )
}

export default ConfirmOutage
