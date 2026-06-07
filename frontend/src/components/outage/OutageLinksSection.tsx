import { Add, Delete, Edit, OpenInNew } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  IconButton,
  Link,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
  styled,
} from '@mui/material'
import { useState } from 'react'
import type { ChangeEvent } from 'react'

import type { OutageLink } from '../../types'
import { outageLinksEndpoint, outageLinkEndpoint } from '../../utils/endpoints'

import Section from './OutageDetailsSection'

interface OutageLinksSectionProps {
  links: OutageLink[]
  isAdmin: boolean
  componentName: string
  subComponentName: string
  outageId: number
  onLinkAdded: (link: OutageLink) => void
  onLinkUpdated: (link: OutageLink) => void
  onLinkDeleted: (linkId: number) => void
  onDeleteSuccess: (message: string) => void
}

const LinkRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  padding: theme.spacing(1, 0),
  '&:not(:last-child)': {
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
}))

const LinkIconBox = styled(Box)(({ theme }) => ({
  color: theme.palette.text.secondary,
  display: 'flex',
  alignItems: 'center',
  flexShrink: 0,
}))

const LinkContent = styled(Box)(() => ({
  flex: 1,
  minWidth: 0,
}))

const AddLinkRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'flex-start',
  gap: theme.spacing(1),
  marginTop: theme.spacing(2),
}))

const LINK_TYPE_OPTIONS = [
  { value: 'incident_channel_thread', label: 'Incident Channel/Thread' },
  { value: 'rca', label: 'RCA' },
  { value: 'other', label: 'Other' },
] as const

const getLinkLabel = (link: OutageLink): string => {
  const option = LINK_TYPE_OPTIONS.find((o) => o.value === link.link_type)
  if (option && link.link_type !== 'other') return option.label
  return link.description || link.url
}

const OutageLinksSection = ({
  links,
  isAdmin,
  componentName,
  subComponentName,
  outageId,
  onLinkAdded,
  onLinkUpdated,
  onLinkDeleted,
  onDeleteSuccess,
}: OutageLinksSectionProps) => {
  const [newLinkURL, setNewLinkURL] = useState('')
  const [newLinkType, setNewLinkType] = useState('incident_channel_thread')
  const [newLinkDesc, setNewLinkDesc] = useState('')
  const [linkLoading, setLinkLoading] = useState(false)
  const [linkError, setLinkError] = useState<string | null>(null)

  const [editingLinkId, setEditingLinkId] = useState<number | null>(null)
  const [editLinkURL, setEditLinkURL] = useState('')
  const [editLinkType, setEditLinkType] = useState('incident_channel_thread')
  const [editLinkDesc, setEditLinkDesc] = useState('')
  const [editLinkLoading, setEditLinkLoading] = useState(false)
  const [editLinkError, setEditLinkError] = useState<string | null>(null)

  const handleAddLink = () => {
    if (!newLinkURL.trim()) return
    setLinkLoading(true)
    setLinkError(null)

    fetch(outageLinksEndpoint(componentName, subComponentName, outageId), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        url: newLinkURL.trim(),
        link_type: newLinkType,
        description: newLinkType === 'other' ? newLinkDesc.trim() : '',
      }),
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((data) => {
            throw new Error(data.error || `HTTP ${response.status}`)
          })
        }
        return response.json()
      })
      .then((link: OutageLink) => {
        setNewLinkURL('')
        setNewLinkType('incident_channel_thread')
        setNewLinkDesc('')
        onLinkAdded(link)
      })
      .catch((err) => {
        setLinkError(err instanceof Error ? err.message : 'Failed to add link')
      })
      .finally(() => {
        setLinkLoading(false)
      })
  }

  const startLinkEdit = (link: OutageLink) => {
    setEditingLinkId(link.ID)
    setEditLinkURL(link.url)
    setEditLinkType(link.link_type ?? 'other')
    setEditLinkDesc(link.description ?? '')
    setEditLinkError(null)
  }

  const cancelLinkEdit = () => {
    setEditingLinkId(null)
    setEditLinkError(null)
  }

  const handleSaveLinkEdit = (linkId: number) => {
    setEditLinkLoading(true)
    setEditLinkError(null)

    fetch(outageLinkEndpoint(componentName, subComponentName, outageId, linkId), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        url: editLinkURL.trim(),
        link_type: editLinkType,
        description: editLinkType === 'other' ? editLinkDesc.trim() : '',
      }),
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((data) => {
            throw new Error(data.error || `HTTP ${response.status}`)
          })
        }
        return response.json()
      })
      .then((updated: OutageLink) => {
        onLinkUpdated(updated)
        setEditingLinkId(null)
      })
      .catch((err) => {
        setEditLinkError(err instanceof Error ? err.message : 'Failed to update link')
      })
      .finally(() => {
        setEditLinkLoading(false)
      })
  }

  const handleDeleteLink = (linkId: number) => {
    fetch(outageLinkEndpoint(componentName, subComponentName, outageId, linkId), {
      method: 'DELETE',
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((data) => {
            throw new Error(data.error || `HTTP ${response.status}`)
          })
        }
        onLinkDeleted(linkId)
        onDeleteSuccess('Link removed')
      })
      .catch((err) => {
        setLinkError(err instanceof Error ? err.message : 'Failed to delete link')
      })
  }

  return (
    <Section icon={<OpenInNew />} title="Links">
      {links.length === 0 && !isAdmin && (
        <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
          No links yet.
        </Typography>
      )}

      {links.map((link) => {
        const isEditing = editingLinkId === link.ID
        return (
          <LinkRow key={link.ID}>
            <LinkIconBox>
              <OpenInNew fontSize="small" />
            </LinkIconBox>
            {isEditing ? (
              <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 1 }}>
                {editLinkError && (
                  <Alert severity="error" sx={{ mb: 0.5 }}>
                    {editLinkError}
                  </Alert>
                )}
                <Box sx={{ display: 'flex', gap: 1 }}>
                  <TextField
                    size="small"
                    label="URL"
                    value={editLinkURL}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => setEditLinkURL(e.target.value)}
                    disabled={editLinkLoading}
                    sx={{ flex: 2 }}
                  />
                  <TextField
                    select
                    size="small"
                    label="Type"
                    value={editLinkType}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => setEditLinkType(e.target.value)}
                    disabled={editLinkLoading}
                    sx={{ flex: 1.5 }}
                  >
                    {LINK_TYPE_OPTIONS.map((o) => (
                      <MenuItem key={o.value} value={o.value}>
                        {o.label}
                      </MenuItem>
                    ))}
                  </TextField>
                </Box>
                {editLinkType === 'other' && (
                  <TextField
                    size="small"
                    label="Description (optional)"
                    value={editLinkDesc}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => setEditLinkDesc(e.target.value)}
                    disabled={editLinkLoading}
                    fullWidth
                  />
                )}
                <Box sx={{ display: 'flex', gap: 1 }}>
                  <Button size="small" onClick={cancelLinkEdit} disabled={editLinkLoading}>
                    Cancel
                  </Button>
                  <Button
                    variant="contained"
                    size="small"
                    startIcon={
                      editLinkLoading ? <CircularProgress size={14} color="inherit" /> : undefined
                    }
                    onClick={() => handleSaveLinkEdit(link.ID)}
                    disabled={editLinkLoading || !editLinkURL.trim()}
                  >
                    {editLinkLoading ? 'Saving...' : 'Save'}
                  </Button>
                </Box>
              </Box>
            ) : (
              <LinkContent>
                <Link
                  href={/^https?:\/\//i.test(link.url) ? link.url : '#'}
                  target="_blank"
                  rel="noopener noreferrer"
                  underline="hover"
                  variant="body2"
                  fontWeight={500}
                  sx={{ wordBreak: 'break-all' }}
                >
                  {getLinkLabel(link)}
                </Link>
                {getLinkLabel(link) !== link.url && (
                  <Typography
                    variant="caption"
                    display="block"
                    color="text.secondary"
                    sx={{ wordBreak: 'break-all' }}
                  >
                    {link.url}
                  </Typography>
                )}
              </LinkContent>
            )}
            {isAdmin && !isEditing && (
              <>
                <Tooltip title="Edit link">
                  <IconButton
                    size="small"
                    onClick={() => startLinkEdit(link)}
                    aria-label="edit link"
                  >
                    <Edit fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Remove link">
                  <IconButton
                    size="small"
                    onClick={() => handleDeleteLink(link.ID)}
                    aria-label="remove link"
                  >
                    <Delete fontSize="small" />
                  </IconButton>
                </Tooltip>
              </>
            )}
          </LinkRow>
        )
      })}

      {isAdmin && (
        <>
          {links.length > 0 && <Divider sx={{ my: 1.5 }} />}
          {linkError && (
            <Alert severity="error" sx={{ mb: 1.5 }}>
              {linkError}
            </Alert>
          )}
          <AddLinkRow>
            <TextField
              size="small"
              label="URL"
              value={newLinkURL}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setNewLinkURL(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newLinkURL.trim()) handleAddLink()
              }}
              disabled={linkLoading}
              sx={{ flex: 2 }}
              placeholder="https://..."
            />
            <TextField
              select
              size="small"
              label="Type"
              value={newLinkType}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setNewLinkType(e.target.value)}
              disabled={linkLoading}
              sx={{ flex: 1.5 }}
            >
              {LINK_TYPE_OPTIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </TextField>
            <Button
              variant="contained"
              size="small"
              startIcon={linkLoading ? <CircularProgress size={14} color="inherit" /> : <Add />}
              onClick={handleAddLink}
              disabled={linkLoading || !newLinkURL.trim()}
              sx={{ height: 40, whiteSpace: 'nowrap', flexShrink: 0 }}
            >
              {linkLoading ? 'Adding...' : 'Add Link'}
            </Button>
          </AddLinkRow>
          {newLinkType === 'other' && (
            <TextField
              size="small"
              label="Description (optional)"
              value={newLinkDesc}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setNewLinkDesc(e.target.value)}
              disabled={linkLoading}
              fullWidth
              sx={{ mt: 1 }}
            />
          )}
        </>
      )}
    </Section>
  )
}

export default OutageLinksSection
