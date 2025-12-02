import { Delete } from '@mui/icons-material'
import { Button, Tooltip } from '@mui/material'
import { useState } from 'react'

import type { Outage } from '../../../types'
import { modifyOutageEndpoint } from '../../../utils/endpoints'

import ConfirmationModal from './ConfirmationModal'

interface DeleteOutageProps {
  outage: Outage
  onDeleteSuccess: () => void
  onError: (error: string) => void
}

const DeleteOutage = ({ outage, onDeleteSuccess, onError }: DeleteOutageProps) => {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const handleDeleteClick = () => {
    setDeleteDialogOpen(true)
  }

  const handleDeleteConfirm = () => {
    setIsLoading(true)
    fetch(modifyOutageEndpoint(outage.component_name, outage.sub_component_name, outage.ID), {
      method: 'DELETE',
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to delete outage: ${response.statusText}`)
        }
        onDeleteSuccess()
      })
      .catch((err) => {
        onError(err instanceof Error ? err.message : 'Failed to delete outage')
      })
      .finally(() => {
        setIsLoading(false)
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
      <ConfirmationModal
        open={deleteDialogOpen}
        onClose={handleDeleteCancel}
        onConfirm={handleDeleteConfirm}
        title="Delete Outage"
        description="Are you sure you want to delete this outage? This action cannot be undone."
        confirmButtonText="Delete"
        confirmButtonColor="error"
        isLoading={isLoading}
        outage={outage}
      />
    </>
  )
}

export default DeleteOutage
