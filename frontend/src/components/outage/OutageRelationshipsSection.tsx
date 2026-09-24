import { Add, Delete, Link as LinkIcon } from '@mui/icons-material'
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

import type { OutageRelationship } from '../../types'
import {
  getOutageRelationshipsEndpoint,
  getOutageRelationshipEndpoint,
} from '../../utils/endpoints'
import { deslugify } from '../../utils/slugify'

import Section from './OutageDetailsSection'

interface OutageRelationshipsSectionProps {
  relationships: OutageRelationship[]
  isAdmin: boolean
  componentName: string
  subComponentName: string
  outageId: number
  onRelationshipAdded: (rel: OutageRelationship) => void
  onRelationshipDeleted: (relId: number) => void
  onDeleteSuccess: (message: string) => void
}

const RelRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  padding: theme.spacing(1, 0),
  '&:not(:last-child)': {
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
}))

const RelIconBox = styled(Box)(({ theme }) => ({
  color: theme.palette.text.secondary,
  display: 'flex',
  alignItems: 'center',
  flexShrink: 0,
}))

const RelContent = styled(Box)(() => ({
  flex: 1,
  minWidth: 0,
}))

const AddRelRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'flex-start',
  gap: theme.spacing(1),
  marginTop: theme.spacing(2),
}))

const RELATIONSHIP_TYPE_OPTIONS = [
  { value: 'causes', label: 'Causes' },
  { value: 'caused_by', label: 'Caused by' },
  { value: 'related_to', label: 'Related to' },
] as const

const getRelationshipLabel = (relType: string): string => {
  const option = RELATIONSHIP_TYPE_OPTIONS.find((o) => o.value === relType)
  return option?.label ?? relType
}

const OutageRelationshipsSection = ({
  relationships,
  isAdmin,
  componentName,
  subComponentName,
  outageId,
  onRelationshipAdded,
  onRelationshipDeleted,
  onDeleteSuccess,
}: OutageRelationshipsSectionProps) => {
  const [newRelOutageId, setNewRelOutageId] = useState('')
  const [newRelType, setNewRelType] = useState('related_to')
  const [relLoading, setRelLoading] = useState(false)
  const [relError, setRelError] = useState<string | null>(null)

  const handleAddRelationship = () => {
    const relatedId = parseInt(newRelOutageId, 10)
    if (isNaN(relatedId) || relatedId <= 0) {
      setRelError('Enter a valid outage ID')
      return
    }
    setRelLoading(true)
    setRelError(null)

    fetch(getOutageRelationshipsEndpoint(componentName, subComponentName, outageId), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        related_outage_id: relatedId,
        relationship_type: newRelType,
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
      .then((rel: OutageRelationship) => {
        setNewRelOutageId('')
        setNewRelType('related_to')
        onRelationshipAdded(rel)
      })
      .catch((err) => {
        setRelError(err instanceof Error ? err.message : 'Failed to add relationship')
      })
      .finally(() => {
        setRelLoading(false)
      })
  }

  const handleDeleteRelationship = (relId: number) => {
    fetch(getOutageRelationshipEndpoint(componentName, subComponentName, outageId, relId), {
      method: 'DELETE',
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((data) => {
            throw new Error(data.error || `HTTP ${response.status}`)
          })
        }
        onRelationshipDeleted(relId)
        onDeleteSuccess('Relationship removed')
      })
      .catch((err) => {
        setRelError(err instanceof Error ? err.message : 'Failed to delete relationship')
      })
  }

  const buildOutageLink = (rel: OutageRelationship): string => {
    if (!rel.related_outage) return '#'
    const comp = rel.related_outage.component_name
    const sub = rel.related_outage.sub_component_name
    return `/${comp}/${sub}/outages/${rel.related_outage_id}`
  }

  const describeRelated = (rel: OutageRelationship): string => {
    if (!rel.related_outage) return `Outage #${rel.related_outage_id}`
    const comp = deslugify(rel.related_outage.component_name)
    const sub = deslugify(rel.related_outage.sub_component_name)
    return `#${rel.related_outage_id}: ${comp} / ${sub}`
  }

  return (
    <Section icon={<LinkIcon />} title="Related Outages">
      {relationships.length === 0 && !isAdmin && (
        <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
          No related outages.
        </Typography>
      )}

      {relationships.map((rel) => (
        <RelRow key={rel.ID}>
          <RelIconBox>
            <LinkIcon fontSize="small" />
          </RelIconBox>
          <RelContent>
            <Typography variant="body2" fontWeight={500} component="span">
              {getRelationshipLabel(rel.relationship_type)}
            </Typography>{' '}
            <Link
              href={buildOutageLink(rel)}
              underline="hover"
              variant="body2"
              sx={{ wordBreak: 'break-all' }}
            >
              {describeRelated(rel)}
            </Link>
            {rel.related_outage?.description && (
              <Typography
                variant="caption"
                display="block"
                color="text.secondary"
                sx={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: '100%',
                }}
              >
                {rel.related_outage.description}
              </Typography>
            )}
          </RelContent>
          {isAdmin && (
            <Tooltip title="Remove relationship">
              <IconButton
                size="small"
                onClick={() => handleDeleteRelationship(rel.ID)}
                aria-label="remove relationship"
              >
                <Delete fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
        </RelRow>
      ))}

      {isAdmin && (
        <>
          {relationships.length > 0 && <Divider sx={{ my: 1.5 }} />}
          {relError && (
            <Alert severity="error" sx={{ mb: 1.5 }}>
              {relError}
            </Alert>
          )}
          <AddRelRow>
            <TextField
              size="small"
              label="Outage ID"
              value={newRelOutageId}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setNewRelOutageId(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newRelOutageId.trim()) handleAddRelationship()
              }}
              disabled={relLoading}
              sx={{ flex: 1 }}
              placeholder="12345"
              type="number"
            />
            <TextField
              select
              size="small"
              label="Relationship"
              value={newRelType}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setNewRelType(e.target.value)}
              disabled={relLoading}
              sx={{ flex: 1.5 }}
            >
              {RELATIONSHIP_TYPE_OPTIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </TextField>
            <Button
              variant="contained"
              size="small"
              startIcon={relLoading ? <CircularProgress size={14} color="inherit" /> : <Add />}
              onClick={handleAddRelationship}
              disabled={relLoading || !newRelOutageId.trim()}
              sx={{ height: 40, whiteSpace: 'nowrap', flexShrink: 0 }}
            >
              {relLoading ? 'Adding...' : 'Add'}
            </Button>
          </AddRelRow>
        </>
      )}
    </Section>
  )
}

export default OutageRelationshipsSection
