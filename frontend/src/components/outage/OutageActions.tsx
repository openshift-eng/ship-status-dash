import { MoreVert, Edit } from '@mui/icons-material'
import { Button, Menu, MenuItem, ListItemIcon, ListItemText, Tooltip } from '@mui/material'
import React, { useState } from 'react'

import DeleteOutage from './DeleteOutage'
import EndOutage from './EndOutage'
import UpsertOutageModal from './UpsertOutageModal'
import type { Outage } from '../../types'

interface OutageActionsProps {
  outage: Outage
  onSuccess: () => void
  onError: (error: string) => void
}

const OutageActions: React.FC<OutageActionsProps> = ({ outage, onSuccess, onError }) => {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false)

  const handleMenuClick = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => {
    setAnchorEl(null)
  }

  const handleUpdateClick = () => {
    setUpdateDialogOpen(true)
    handleMenuClose()
  }

  const handleUpdateClose = () => {
    setUpdateDialogOpen(false)
  }

  return (
    <>
      <Tooltip title="Outage actions" arrow>
        <Button size="small" onClick={handleMenuClick} startIcon={<MoreVert />}>
          Actions
        </Button>
      </Tooltip>

      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleMenuClose}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
      >
        <MenuItem onClick={handleUpdateClick}>
          <ListItemIcon>
            <Edit fontSize="small" />
          </ListItemIcon>
          <ListItemText>Update</ListItemText>
        </MenuItem>
        {!outage.end_time.Valid && (
          <MenuItem>
            <EndOutage outage={outage} onEndSuccess={onSuccess} onError={onError} />
          </MenuItem>
        )}
        <MenuItem>
          <DeleteOutage outage={outage} onDeleteSuccess={onSuccess} onError={onError} />
        </MenuItem>
      </Menu>

      <UpsertOutageModal
        open={updateDialogOpen}
        onClose={handleUpdateClose}
        onSuccess={onSuccess}
        componentName={outage.component_name}
        subComponentName={outage.sub_component_name}
        outage={outage}
      />
    </>
  )
}

export default OutageActions
