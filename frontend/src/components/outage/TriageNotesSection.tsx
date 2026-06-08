import { Add, Delete, Edit } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  IconButton,
  TextField,
  Tooltip,
  Typography,
  styled,
} from '@mui/material'
import type { ChangeEvent, KeyboardEvent } from 'react'
import { useState } from 'react'

import type { TriageNote } from '../../types'
import { getTriageNoteEndpoint, getTriageNotesEndpoint } from '../../utils/endpoints'
import { relativeTime } from '../../utils/helpers'

const NoteList = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(1.5),
}))

const NoteItem = styled(Box)(({ theme }) => ({
  padding: theme.spacing(1.5),
  borderRadius: theme.spacing(1),
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[50],
  borderLeft: `3px solid ${theme.palette.primary.main}`,
}))

const NoteHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(0.5),
  flexWrap: 'wrap',
}))

const NoteAuthor = styled(Typography)(() => ({
  fontWeight: 600,
  fontSize: '0.875rem',
}))

const NoteTimestamp = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.75rem',
}))

const NoteBody = styled(Typography)(() => ({
  whiteSpace: 'pre-wrap',
  fontSize: '0.9375rem',
  lineHeight: 1.6,
}))

const NoteActions = styled(Box)(({ theme }) => ({
  marginLeft: 'auto',
  display: 'flex',
  gap: theme.spacing(0.5),
}))

const EditActions = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  gap: theme.spacing(1),
  marginTop: theme.spacing(1),
}))

const ComposeArea = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(2),
}))

const ComposeActions = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  marginTop: theme.spacing(1),
}))

const EmptyNotice = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontStyle: 'italic',
  fontSize: '0.875rem',
  marginBottom: theme.spacing(2),
}))

interface TriageNotesSectionProps {
  notes: TriageNote[]
  isAdmin: boolean
  currentUser: string
  componentName: string
  subComponentName: string
  outageId: number
  onNoteAdded: (note: TriageNote) => void
  onNoteUpdated: (note: TriageNote) => void
  onNoteDeleted: (noteId: number) => void
  onDeleteSuccess: (message: string) => void
}

const TriageNotesSection = ({
  notes,
  isAdmin,
  currentUser,
  componentName,
  subComponentName,
  outageId,
  onNoteAdded,
  onNoteUpdated,
  onNoteDeleted,
  onDeleteSuccess,
}: TriageNotesSectionProps) => {
  const [body, setBody] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [editingNoteId, setEditingNoteId] = useState<number | null>(null)
  const [editBody, setEditBody] = useState('')
  const [editLoading, setEditLoading] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)

  const handleSubmit = () => {
    if (!body.trim()) return

    setLoading(true)
    setError(null)

    fetch(getTriageNotesEndpoint(componentName, subComponentName, outageId), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ body: body.trim() }),
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
      .then((note: TriageNote) => {
        setBody('')
        onNoteAdded(note)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to add note')
      })
      .finally(() => {
        setLoading(false)
      })
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      handleSubmit()
    }
  }

  const startEdit = (note: TriageNote) => {
    setEditingNoteId(note.ID)
    setEditBody(note.body)
    setEditError(null)
  }

  const cancelEdit = () => {
    setEditingNoteId(null)
    setEditBody('')
    setEditError(null)
  }

  const handleSaveEdit = (noteId: number) => {
    if (!editBody.trim()) return

    setEditLoading(true)
    setEditError(null)

    fetch(getTriageNoteEndpoint(componentName, subComponentName, outageId, noteId), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ body: editBody.trim() }),
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
      .then((note: TriageNote) => {
        setEditingNoteId(null)
        setEditBody('')
        onNoteUpdated(note)
      })
      .catch((err) => {
        setEditError(err instanceof Error ? err.message : 'Failed to update note')
      })
      .finally(() => {
        setEditLoading(false)
      })
  }

  const handleDelete = (noteId: number) => {
    fetch(getTriageNoteEndpoint(componentName, subComponentName, outageId, noteId), {
      method: 'DELETE',
      credentials: 'include',
    })
      .then((response) => {
        if (!response.ok) {
          return response.json().then((data) => {
            throw new Error(data.error || `HTTP ${response.status}`)
          })
        }
        onNoteDeleted(noteId)
        onDeleteSuccess('Triage note deleted')
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to delete note')
      })
  }

  return (
    <Box>
      {error && (
        <Alert severity="error" sx={{ mb: 1.5 }}>
          {error}
        </Alert>
      )}

      {notes.length === 0 && !isAdmin && (
        <EmptyNotice variant="body2">No triage notes yet.</EmptyNotice>
      )}

      {notes.length > 0 && (
        <NoteList>
          {notes.map((note) => {
            const canModify = isAdmin || note.author === currentUser
            const isEditing = editingNoteId === note.ID

            return (
              <NoteItem key={note.ID}>
                <NoteHeader>
                  <NoteAuthor variant="body2">{note.author}</NoteAuthor>
                  <NoteTimestamp variant="caption">
                    {relativeTime(new Date(note.CreatedAt), new Date())}
                  </NoteTimestamp>
                  {canModify && !isEditing && (
                    <NoteActions>
                      <Tooltip title="Edit note">
                        <IconButton
                          size="small"
                          onClick={() => startEdit(note)}
                          aria-label="edit note"
                        >
                          <Edit fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete note">
                        <IconButton
                          size="small"
                          onClick={() => handleDelete(note.ID)}
                          aria-label="delete note"
                        >
                          <Delete fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </NoteActions>
                  )}
                </NoteHeader>

                {isEditing ? (
                  <>
                    {editError && (
                      <Alert severity="error" sx={{ mb: 1 }}>
                        {editError}
                      </Alert>
                    )}
                    <TextField
                      multiline
                      rows={3}
                      value={editBody}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setEditBody(e.target.value)}
                      disabled={editLoading}
                      size="small"
                      fullWidth
                      autoFocus
                    />
                    <EditActions>
                      <Button size="small" onClick={cancelEdit} disabled={editLoading}>
                        Cancel
                      </Button>
                      <Button
                        variant="contained"
                        size="small"
                        startIcon={
                          editLoading ? <CircularProgress size={14} color="inherit" /> : undefined
                        }
                        onClick={() => handleSaveEdit(note.ID)}
                        disabled={editLoading || !editBody.trim()}
                      >
                        {editLoading ? 'Saving...' : 'Save'}
                      </Button>
                    </EditActions>
                  </>
                ) : (
                  <NoteBody variant="body2">{note.body}</NoteBody>
                )}
              </NoteItem>
            )
          })}
        </NoteList>
      )}

      {isAdmin && (
        <>
          {notes.length > 0 && <Divider sx={{ my: 2 }} />}
          <ComposeArea>
            <TextField
              multiline
              rows={3}
              placeholder="Add a triage note..."
              value={body}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setBody(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={loading}
              size="small"
              fullWidth
            />
            <ComposeActions>
              <Button
                variant="contained"
                size="small"
                startIcon={loading ? <CircularProgress size={14} color="inherit" /> : <Add />}
                onClick={handleSubmit}
                disabled={loading || !body.trim()}
              >
                {loading ? 'Posting...' : 'Post Note'}
              </Button>
            </ComposeActions>
          </ComposeArea>
        </>
      )}
    </Box>
  )
}

export default TriageNotesSection
