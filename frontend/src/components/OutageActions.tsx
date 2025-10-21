import { MoreVert, Edit, Visibility } from '@mui/icons-material'
import { Button, Menu, MenuItem, ListItemIcon, ListItemText, Tooltip } from '@mui/material'
import React, { useState } from 'react'

import UpsertOutageModal from './UpsertOutageModal'
import DeleteOutage from './DeleteOutage'
import OutageDetailsModal from './OutageDetailsModal'
import type { Outage } from '../types'

interface OutageActionsProps {
  outage: Outage
  onSuccess: () => void
  onError: (error: string) => void
}

const OutageActions: React.FC<OutageActionsProps> = ({ outage, onSuccess, onError }) => {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false)
  const [detailsDialogOpen, setDetailsDialogOpen] = useState(false)

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

  const handleDetailsClick = () => {
    setDetailsDialogOpen(true)
    handleMenuClose()
  }

  const handleDetailsClose = () => {
    setDetailsDialogOpen(false)
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
        <MenuItem onClick={handleDetailsClick}>
          <ListItemIcon>
            <Visibility fontSize="small" />
          </ListItemIcon>
          <ListItemText>Full Details</ListItemText>
        </MenuItem>
        <MenuItem onClick={handleUpdateClick}>
          <ListItemIcon>
            <Edit fontSize="small" />
          </ListItemIcon>
          <ListItemText>Update</ListItemText>
        </MenuItem>
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

      <OutageDetailsModal
        open={detailsDialogOpen}
        onClose={handleDetailsClose}
        outage={outage}
      />
    </>
  )
}

export default OutageActions
