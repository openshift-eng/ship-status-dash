import History from '@mui/icons-material/History'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
  styled,
} from '@mui/material'
import { alpha } from '@mui/material/styles'
import * as Diff from 'diff'
import { useCallback, useEffect, useState } from 'react'

import type { OutageAuditLog } from '../../types'
import { getOutageAuditLogsEndpoint } from '../../utils/endpoints'

const StyledDialog = styled(Dialog)(({ theme }) => ({
  '& .MuiDialog-paper': {
    borderRadius: theme.spacing(2),
    maxWidth: 720,
  },
}))

const LogEntry = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(3),
  '&:last-of-type': { marginBottom: 0 },
}))

const LogHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  flexWrap: 'wrap',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(1),
}))

const DiffBlock = styled(Box)(({ theme }) => ({
  padding: theme.spacing(1.5),
  borderRadius: theme.spacing(1),
  border: `1px solid ${theme.palette.divider}`,
  overflow: 'auto',
  maxHeight: 320,
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[900] : theme.palette.grey[50],
}))

const DiffLine = styled('div')<{ variant?: 'add' | 'remove' | 'unchanged' }>(({
  theme,
  variant = 'unchanged',
}) => {
  const addColor = theme.palette.diff?.add.main ?? theme.palette.success.main
  const removeColor = theme.palette.diff?.remove.main ?? theme.palette.error.main
  const bg =
    variant === 'add'
      ? alpha(addColor, theme.palette.mode === 'dark' ? 0.25 : 0.15)
      : variant === 'remove'
        ? alpha(removeColor, theme.palette.mode === 'dark' ? 0.25 : 0.15)
        : 'transparent'
  const borderLeft =
    variant === 'add'
      ? `3px solid ${addColor}`
      : variant === 'remove'
        ? `3px solid ${removeColor}`
        : '3px solid transparent'
  return {
    fontFamily: 'monospace',
    fontSize: '0.8125rem',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    padding: theme.spacing(0.25, 0.5),
    margin: 0,
    backgroundColor: bg,
    borderLeft,
    color: theme.palette.text.primary,
  }
})

const ScrollContent = styled(Box)(({ theme }) => ({
  maxHeight: '70vh',
  overflowY: 'auto',
  paddingRight: theme.spacing(1),
}))

function decodeJsonPayload(raw: string | undefined): string {
  if (!raw) return ''
  try {
    const decoded = atob(raw)
    const parsed = JSON.parse(decoded) as unknown
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}

type DiffLineVariant = 'add' | 'remove' | 'unchanged'

function getUnifiedDiffLines(
  oldStr: string,
  newStr: string,
): Array<{ variant: DiffLineVariant; line: string }> {
  const changes = Diff.diffLines(oldStr, newStr)
  const lines: Array<{ variant: DiffLineVariant; line: string }> = []
  for (const change of changes) {
    const variant: DiffLineVariant = change.added ? 'add' : change.removed ? 'remove' : 'unchanged'
    const lineStrings = change.value.split('\n')
    if (lineStrings[lineStrings.length - 1] === '') lineStrings.pop()
    for (const line of lineStrings) {
      lines.push({ variant, line })
    }
  }
  return lines
}

interface AuditLogModalProps {
  open: boolean
  onClose: () => void
  componentName: string
  subComponentName: string
  outageId: number
}

const AuditLogModal = ({
  open,
  onClose,
  componentName,
  subComponentName,
  outageId,
}: AuditLogModalProps) => {
  const [logs, setLogs] = useState<OutageAuditLog[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchLogs = useCallback(() => {
    setLoading(true)
    setError(null)
    fetch(getOutageAuditLogsEndpoint(componentName, subComponentName, outageId))
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText)
        return res.json()
      })
      .then(setLogs)
      .catch((err) => setError(err.message || 'Failed to load audit logs'))
      .finally(() => setLoading(false))
  }, [componentName, subComponentName, outageId])

  useEffect(() => {
    if (open) fetchLogs()
  }, [open, fetchLogs])

  const formatDateTime = (dateString: string) => new Date(dateString).toLocaleString()

  const getOperationColor = (op: string): 'success' | 'info' | 'error' => {
    if (op === 'CREATE') return 'success'
    if (op === 'UPDATE') return 'info'
    return 'error'
  }

  return (
    <StyledDialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <History /> Audit Logs
      </DialogTitle>
      <DialogContent dividers>
        {loading && (
          <Box display="flex" justifyContent="center" py={4}>
            <CircularProgress />
          </Box>
        )}
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}
        {!loading && !error && logs.length === 0 && (
          <Typography color="text.secondary">No audit entries.</Typography>
        )}
        {!loading && !error && logs.length > 0 && (
          <ScrollContent>
            {logs.map((log) => (
              <LogEntry key={log.ID}>
                <LogHeader>
                  <Chip
                    label={log.operation}
                    color={getOperationColor(log.operation)}
                    size="small"
                  />
                  <Typography variant="body2" color="text.secondary">
                    {log.user}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {formatDateTime(log.CreatedAt)}
                  </Typography>
                </LogHeader>
                {log.operation === 'CREATE' &&
                  (() => {
                    const newStr = decodeJsonPayload(log.new)
                    const diffLines = getUnifiedDiffLines('', newStr)
                    return (
                      <DiffBlock>
                        {diffLines.map(({ variant, line }, i) => (
                          <DiffLine key={i} variant={variant}>
                            {variant === 'add' ? '+' : ''}
                            {line}
                          </DiffLine>
                        ))}
                      </DiffBlock>
                    )
                  })()}
                {log.operation === 'UPDATE' &&
                  (() => {
                    const oldStr = decodeJsonPayload(log.old)
                    const newStr = decodeJsonPayload(log.new)
                    const diffLines = getUnifiedDiffLines(oldStr, newStr)
                    return (
                      <DiffBlock>
                        {diffLines.map(({ variant, line }, i) => (
                          <DiffLine key={i} variant={variant}>
                            {variant === 'add' ? '+' : variant === 'remove' ? '-' : ' '}
                            {line}
                          </DiffLine>
                        ))}
                      </DiffBlock>
                    )
                  })()}
                {log.operation === 'DELETE' &&
                  (() => {
                    const oldStr = decodeJsonPayload(log.old)
                    const diffLines = getUnifiedDiffLines(oldStr, '')
                    return (
                      <DiffBlock>
                        {diffLines.map(({ variant, line }, i) => (
                          <DiffLine key={i} variant={variant}>
                            {variant === 'remove' ? '-' : ''}
                            {line}
                          </DiffLine>
                        ))}
                      </DiffBlock>
                    )
                  })()}
              </LogEntry>
            ))}
          </ScrollContent>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </StyledDialog>
  )
}

export default AuditLogModal
