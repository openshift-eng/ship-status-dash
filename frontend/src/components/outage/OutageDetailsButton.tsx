import { Visibility } from '@mui/icons-material'
import { Button, Tooltip } from '@mui/material'
import React, { useState } from 'react'

import OutageDetailsModal from './OutageDetailsModal'
import type { Outage } from '../../types'

interface OutageDetailsButtonProps {
  outage: Outage
}

const OutageDetailsButton: React.FC<OutageDetailsButtonProps> = ({ outage }) => {
  const [detailsDialogOpen, setDetailsDialogOpen] = useState(false)

  const handleDetailsClick = () => {
    setDetailsDialogOpen(true)
  }

  const handleDetailsClose = () => {
    setDetailsDialogOpen(false)
  }

  return (
    <>
      <Tooltip title="View full details" arrow>
        <Button size="small" onClick={handleDetailsClick} startIcon={<Visibility />}></Button>
      </Tooltip>

      <OutageDetailsModal open={detailsDialogOpen} onClose={handleDetailsClose} outage={outage} />
    </>
  )
}

export default OutageDetailsButton
