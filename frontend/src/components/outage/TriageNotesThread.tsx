import { Add } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  TextField,
  Typography,
  styled,
} from '@mui/material'
import type { ChangeEvent, KeyboardEvent } from 'react'
import { useState } from 'react'

import type { TriageNote } from '../../types'
import { addTriageNoteEndpoint } from '../../utils/endpoints'
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
  alignItems: 'baseline',
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

interface TriageNotesThreadProps {
  notes: TriageNote[]
  isAdmin: boolean
  componentName: string
  subComponentName: string
  outageId: number
  onNoteAdded: (note: TriageNote) => void
}

const TriageNotesThread = ({
  notes,
  isAdmin,
  componentName,
  subComponentName,
  outageId,
  onNoteAdded,
}: TriageNotesThreadProps) => {
  const [body, setBody] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = () => {
    if (!body.trim()) return

    setLoading(true)
    setError(null)

    fetch(addTriageNoteEndpoint(componentName, subComponentName, outageId), {
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

  return (
    <Box>
      {notes.length === 0 && !isAdmin && (
        <EmptyNotice variant="body2">No triage notes yet.</EmptyNotice>
      )}

      {notes.length > 0 && (
        <NoteList>
          {notes.map((note) => (
            <NoteItem key={note.ID}>
              <NoteHeader>
                <NoteAuthor variant="body2">{note.author}</NoteAuthor>
                <NoteTimestamp variant="caption">
                  {relativeTime(new Date(note.CreatedAt), new Date())}
                </NoteTimestamp>
              </NoteHeader>
              <NoteBody variant="body2">{note.body}</NoteBody>
            </NoteItem>
          ))}
        </NoteList>
      )}

      {isAdmin && (
        <>
          {notes.length > 0 && <Divider sx={{ my: 2 }} />}
          <ComposeArea>
            {error && (
              <Alert severity="error" sx={{ mb: 1.5 }}>
                {error}
              </Alert>
            )}
            <TextField
              multiline
              rows={3}
              placeholder="Add a triage note... (Ctrl+Enter to post)"
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

export default TriageNotesThread
