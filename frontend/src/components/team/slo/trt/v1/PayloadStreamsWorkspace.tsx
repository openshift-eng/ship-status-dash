import CheckCircle from '@mui/icons-material/CheckCircle'
import ExpandLess from '@mui/icons-material/ExpandLess'
import ExpandMore from '@mui/icons-material/ExpandMore'
import { Box, Button, Card, Chip, Link, styled, Typography } from '@mui/material'
import { useState } from 'react'

import type {
  SLOEvaluation,
  SLOItem,
  SLOJob,
  SLOSharedCause,
  SLOWorkspace,
} from '../../../../../types'
import { getStatusChipColor } from '../../../../../utils/helpers'
import { slugify } from '../../../../../utils/slugify'
import { getStatusTintStyles } from '../../../../../utils/styles'
import { finishedLabel, formatAge, linkLabel, streamDomId } from '../../format'

import { trtPayloadResult, trtPayloadSettings } from './result'
import UpsertPayloadItemDialog from './UpsertPayloadItemDialog'

const streamPayloads = (rows: SLOItem[], stream: string): SLOItem[] =>
  rows
    .filter((item) => item.group_key === stream)
    .sort((a, b) => new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime())

const payloadTintStatus = (outcome?: string): string | undefined => {
  switch (outcome) {
    case 'Accepted':
      return 'Healthy'
    case 'Rejected':
      return 'Down'
    default:
      return outcome ? 'Unknown' : undefined
  }
}

const Header = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  gap: theme.spacing(2),
  marginBottom: theme.spacing(2),
}))

const StreamWell = styled(Card, {
  shouldForwardProp: (prop) => prop !== 'status',
})<{ status?: string }>(({ theme, status }) => ({
  ...(status
    ? getStatusTintStyles(theme, status, 2)
    : { backgroundColor: theme.palette.background.paper }),
  borderRadius: theme.spacing(2),
  padding: theme.spacing(3),
  marginBottom: theme.spacing(3),
  border: status
    ? `1px solid ${getStatusChipColor(theme, status)}66`
    : `1px solid ${theme.palette.divider}`,
  scrollMarginTop: theme.spacing(10),
}))

const StreamHead = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(1),
}))

const PayloadList = styled(Box)({
  display: 'flex',
  flexDirection: 'column',
})

type RowTone = 'healthy' | 'down'

const PayloadRow = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'tone',
})<{ tone?: RowTone }>(({ theme, tone }) => ({
  display: 'grid',
  gridTemplateColumns: 'minmax(240px, 0.7fr) minmax(0, 1.5fr)',
  gap: theme.spacing(3),
  alignItems: tone === 'healthy' ? 'center' : 'start',
  padding: tone ? theme.spacing(1.5) : theme.spacing(1.5, 0),
  borderBottom: tone ? 'none' : `1px solid ${theme.palette.divider}`,
  borderRadius: tone ? theme.spacing(1) : 0,
  margin: tone ? theme.spacing(1, 0) : 0,
  ...(tone === 'healthy' ? getStatusTintStyles(theme, 'Healthy', 1) : {}),
  ...(tone === 'down' ? getStatusTintStyles(theme, 'Down', 1) : {}),
  '@media (max-width: 800px)': {
    gridTemplateColumns: '1fr',
  },
}))

const FactPanel = styled(Box)(({ theme }) => ({
  backgroundColor:
    theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[100],
  border: `1px solid ${theme.palette.divider}`,
  borderRadius: theme.spacing(1),
  padding: theme.spacing(1.5, 2),
}))

const FactList = styled('dl')(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'auto minmax(0, 1fr)',
  margin: 0,
  alignItems: 'start',
  '& > dt, & > dd': {
    paddingBottom: theme.spacing(1.25),
  },
  '& > dt:not(:first-of-type), & > dd:not(:first-of-type)': {
    borderTop: `1px solid ${theme.palette.divider}`,
    paddingTop: theme.spacing(1.25),
  },
  '& > dt:last-of-type, & > dd:last-of-type': {
    paddingBottom: 0,
  },
}))

const FactLabel = styled('dt')(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontWeight: 600,
  fontSize: '0.8125rem',
  whiteSpace: 'nowrap',
  paddingRight: theme.spacing(2),
}))

const FactValue = styled('dd')({
  margin: 0,
  minWidth: 0,
})

const PhaseValue = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  alignItems: 'center',
  gap: theme.spacing(1),
}))

const EditAction = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(1.5),
}))

const JobNote = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.75rem',
}))

const PayloadNote = styled(Typography)({
  fontSize: '0.875rem',
  whiteSpace: 'pre-wrap',
})

const JobsToggle = styled(Button)(({ theme }) => ({
  justifyContent: 'flex-start',
  marginBottom: theme.spacing(0.75),
  minWidth: 0,
  padding: 0,
  textTransform: 'none',
  fontWeight: 600,
  color: theme.palette.text.primary,
  '&:hover': {
    backgroundColor: 'transparent',
  },
}))

const SectionLabel = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.65rem',
  fontWeight: 700,
  letterSpacing: '0.06em',
  textTransform: 'uppercase',
  marginBottom: theme.spacing(0.75),
}))

const FieldLabel = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  display: 'block',
  fontSize: '0.7rem',
  marginBottom: theme.spacing(0.25),
}))

const CauseList = styled(Box)(({ theme }) => ({
  border: `1px solid ${theme.palette.divider}`,
  borderRadius: theme.spacing(1),
  overflow: 'hidden',
}))

const CauseRow = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'highlighted',
})<{ highlighted?: boolean }>(({ theme, highlighted }) => ({
  display: 'grid',
  gridTemplateColumns: 'minmax(0, 1fr) auto',
  gap: theme.spacing(1),
  alignItems: 'center',
  padding: theme.spacing(1),
  backgroundColor: theme.palette.action.hover,
  outline: highlighted ? `2px solid ${theme.palette.primary.main}` : 'none',
  '& + &': {
    borderTop: `1px solid ${theme.palette.divider}`,
  },
}))

const CauseMeta = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(1),
  marginTop: theme.spacing(0.5),
}))

const ClearJobs = styled(Box)({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  height: '100%',
})

const ClearJobsIcon = styled(CheckCircle)(({ theme }) => ({
  fontSize: theme.spacing(10),
  color: theme.palette.success.main,
  opacity: 0.35,
}))

const JobList = styled(Box)(({ theme }) => ({
  border: `1px solid ${theme.palette.divider}`,
  borderRadius: theme.spacing(1),
  overflow: 'hidden',
}))

const JobRow = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'minmax(140px, 0.85fr) minmax(120px, 0.75fr) minmax(160px, 1.2fr)',
  gap: theme.spacing(1.5),
  alignItems: 'start',
  padding: theme.spacing(1),
  '& + &': {
    borderTop: `1px solid ${theme.palette.divider}`,
  },
  '@media (max-width: 720px)': {
    gridTemplateColumns: '1fr',
  },
}))

const CauseRefs = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(0.5),
}))

const StreakChip = styled(Chip)(({ theme }) => ({
  marginLeft: theme.spacing(1),
}))

const PassLine = styled(Box)(({ theme }) => ({
  gridColumn: '1 / -1',
  display: 'flex',
  flexWrap: 'wrap',
  alignItems: 'center',
  gap: theme.spacing(0.75),
}))

const causeDomId = (itemKey: string, noteId: string, index: number) =>
  `cause-${slugify(itemKey) || 'payload'}-${index}-${slugify(noteId) || 'cause'}`

const causeLinks = (note: SLOSharedCause) => {
  if (note.links && note.links.length > 0) {
    return note.links
  }
  if (note.url) {
    return [{ label: 'Link', url: note.url }]
  }
  return []
}

const jobsReferencing = (jobs: SLOJob[], noteId: string) =>
  jobs.filter((job) => (job.note_ids ?? []).includes(noteId)).length

interface FailedJobsProps {
  itemKey: string
  jobs: SLOJob[]
  notes: SLOSharedCause[]
  highlightedCause: string
  onFocusCause: (domId: string) => void
}

const failedJobsLabel = (count: number) =>
  `${count} blocking ${count === 1 ? 'job' : 'jobs'} failed`

const FailedJobs = ({ itemKey, jobs, notes, highlightedCause, onFocusCause }: FailedJobsProps) => {
  const causesById = new Map(notes.map((note) => [note.id, note]))
  const [jobsOpen, setJobsOpen] = useState(jobs.length <= 3)
  return (
    <>
      {jobs.length > 0 ? (
        <Box>
          <JobsToggle
            aria-expanded={jobsOpen}
            startIcon={jobsOpen ? <ExpandLess /> : <ExpandMore />}
            onClick={() => setJobsOpen((open) => !open)}
          >
            {jobsOpen ? 'Failed blocking jobs' : failedJobsLabel(jobs.length)}
          </JobsToggle>
          {jobsOpen && (
            <JobList>
              {jobs.map((job) => {
                const noteIDs = job.note_ids ?? []
                return (
                  <JobRow key={job.name}>
                    <div>
                      <Link
                        href={job.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        fontWeight={600}
                      >
                        {job.name}
                      </Link>
                      {job.recurring_count !== undefined && job.recurring_count >= 2 && (
                        <StreakChip
                          size="small"
                          color="warning"
                          label={`${job.recurring_count} Payload Streak`}
                        />
                      )}
                    </div>
                    <div>
                      {notes.length > 0 && <FieldLabel>Root causes</FieldLabel>}
                      {noteIDs.length > 0 ? (
                        <CauseRefs>
                          {noteIDs.map((noteId) => {
                            const cause = causesById.get(noteId)
                            if (!cause) {
                              return (
                                <Chip
                                  key={noteId}
                                  size="small"
                                  label={`${noteId} unavailable`}
                                  aria-label={`Root cause ${noteId} unavailable`}
                                />
                              )
                            }
                            const domId = causeDomId(
                              itemKey,
                              cause.id,
                              notes.findIndex((item) => item.id === cause.id),
                            )
                            return (
                              <Chip
                                key={noteId}
                                size="small"
                                component="button"
                                type="button"
                                clickable
                                label={noteId}
                                aria-label={`Root cause ${noteId}: ${cause.text}`}
                                onClick={() => onFocusCause(domId)}
                              />
                            )
                          })}
                        </CauseRefs>
                      ) : (
                        notes.length > 0 && <JobNote>Cause pending</JobNote>
                      )}
                    </div>
                    <div>{job.notes && <JobNote>{job.notes}</JobNote>}</div>
                    {job.later_pass && (
                      <PassLine>
                        <Chip size="small" color="success" variant="outlined" label="Later pass" />
                        <JobNote component="span">Passed on {job.later_pass.tag}</JobNote>
                        <Link href={job.later_pass.url} target="_blank" rel="noopener noreferrer">
                          Prow run
                        </Link>
                      </PassLine>
                    )}
                  </JobRow>
                )
              })}
            </JobList>
          )}
        </Box>
      ) : (
        <ClearJobs>
          <ClearJobsIcon role="img" aria-label="No failed blocking jobs" />
        </ClearJobs>
      )}
      {notes.length > 0 && (
        <Box marginTop={jobs.length > 0 ? 1.5 : 0}>
          <SectionLabel>Shared root causes</SectionLabel>
          <CauseList>
            {notes.map((note, index) => {
              const domId = causeDomId(itemKey, note.id, index)
              const count = jobsReferencing(jobs, note.id)
              const links = causeLinks(note)
              return (
                <CauseRow
                  key={note.id}
                  id={domId}
                  tabIndex={-1}
                  highlighted={highlightedCause === domId}
                >
                  <div>
                    <Typography component="span" fontWeight={700} fontSize="0.75rem">
                      {note.id}
                    </Typography>
                    <Typography
                      component="span"
                      fontWeight={600}
                      fontSize="0.8125rem"
                      marginLeft={0.75}
                    >
                      {note.text}
                    </Typography>
                    {links.length > 0 && (
                      <CauseMeta>
                        {links.map((link) => (
                          <Link
                            key={`${link.label}-${link.url}`}
                            href={link.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            fontSize="0.75rem"
                          >
                            {link.label}
                          </Link>
                        ))}
                      </CauseMeta>
                    )}
                  </div>
                  <Chip size="small" label={count === 1 ? '1 job' : `${count} jobs`} />
                </CauseRow>
              )
            })}
          </CauseList>
        </Box>
      )}
    </>
  )
}

export interface PayloadStreamsWorkspaceProps {
  team: string
  workspace: SLOWorkspace
  evaluations: SLOEvaluation[]
  items: SLOItem[]
  canEdit: boolean
  onChanged: () => void
}

const PayloadStreamsWorkspace = ({
  team,
  workspace,
  evaluations,
  items,
  canEdit,
  onChanged,
}: PayloadStreamsWorkspaceProps) => {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<SLOItem | undefined>(undefined)
  const [highlightedCause, setHighlightedCause] = useState('')
  const focusCause = (domId: string) => {
    const node = document.getElementById(domId)
    node?.scrollIntoView({ block: 'nearest' })
    node?.focus()
    setHighlightedCause(domId)
  }
  const settings = trtPayloadSettings(workspace)
  const result = trtPayloadResult(evaluations[0])
  if (!settings) {
    return null
  }
  const groups = result ? result.groups : []
  const streams = settings.streams

  return (
    <Box>
      <Header>
        <Typography variant="h6">Payload streams (amd64)</Typography>
        {canEdit && (
          <Button
            variant="outlined"
            onClick={() => {
              setEditing(undefined)
              setDialogOpen(true)
            }}
          >
            Add payload
          </Button>
        )}
      </Header>
      {streams.map((stream) => {
        const group = groups.find((item) => item.key === stream.name)
        const rows = streamPayloads(items, stream.name).slice(0, settings.recent_payloads)
        return (
          <StreamWell
            key={stream.name}
            id={streamDomId(stream.name)}
            elevation={0}
            status={payloadTintStatus(rows[0]?.outcome)}
          >
            <StreamHead>
              <Typography fontWeight={600}>{stream.name}</Typography>
              {group && (
                <Chip
                  size="small"
                  color={group.met ? 'success' : 'warning'}
                  label={`${group.met ? 'SLO met' : 'SLO missed'} · ${
                    group.last_accepted_at
                      ? `last accepted ${formatAge(group.last_accepted_at)} ago`
                      : 'no accepted payload stored'
                  }`}
                />
              )}
            </StreamHead>
            <PayloadList>
              {rows.length === 0 && <Typography>No payloads stored</Typography>}
              {rows.map((row) => {
                const finished = row.details.finished_at
                  ? finishedLabel(row.details.finished_at)
                  : ''
                const clear = (row.details.jobs ?? []).length === 0
                const tone: RowTone | undefined =
                  row.outcome === 'Rejected' ? 'down' : clear ? 'healthy' : undefined
                return (
                  <PayloadRow key={row.item_key} tone={tone}>
                    <FactPanel>
                      <FactList>
                        <FactLabel>Payload</FactLabel>
                        <FactValue>
                          <div>{row.item_key}</div>
                          {row.details.payload_url && (
                            <div>
                              <Link
                                href={row.details.payload_url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                Release controller
                              </Link>
                            </div>
                          )}
                          {row.details.analysis_url && (
                            <div>
                              <Link
                                href={row.details.analysis_url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                Payload agent
                              </Link>
                            </div>
                          )}
                        </FactValue>
                        <FactLabel>Phase</FactLabel>
                        <FactValue>
                          <PhaseValue>
                            <Chip
                              size="small"
                              label={row.outcome}
                              color={row.outcome === 'Accepted' ? 'success' : 'error'}
                            />
                            {finished && <JobNote component="span">{finished}</JobNote>}
                          </PhaseValue>
                        </FactValue>
                        <FactLabel>Links</FactLabel>
                        <FactValue>
                          {row.links.length === 0 && '-'}
                          {row.links.map((link) => (
                            <div key={link.ID}>
                              <Link href={link.url} target="_blank" rel="noopener noreferrer">
                                {linkLabel(link)}
                              </Link>
                            </div>
                          ))}
                        </FactValue>
                        {(row.notes ?? '').trim() !== '' && (
                          <>
                            <FactLabel>Note</FactLabel>
                            <FactValue>
                              <PayloadNote>{row.notes}</PayloadNote>
                            </FactValue>
                          </>
                        )}
                      </FactList>
                      {canEdit && (
                        <EditAction>
                          <Button
                            variant="outlined"
                            color="primary"
                            size="small"
                            onClick={() => {
                              setEditing(row)
                              setDialogOpen(true)
                            }}
                          >
                            Edit
                          </Button>
                        </EditAction>
                      )}
                    </FactPanel>
                    <Box>
                      <FailedJobs
                        itemKey={row.item_key}
                        jobs={row.details.jobs}
                        notes={row.details.shared_causes ?? []}
                        highlightedCause={highlightedCause}
                        onFocusCause={focusCause}
                      />
                    </Box>
                  </PayloadRow>
                )
              })}
            </PayloadList>
          </StreamWell>
        )
      })}
      {canEdit && (
        <UpsertPayloadItemDialog
          key={`${dialogOpen}-${editing?.item_key ?? 'new'}`}
          open={dialogOpen}
          team={team}
          streams={streams.map((stream) => stream.name)}
          item={editing}
          onClose={() => setDialogOpen(false)}
          onSuccess={() => {
            setDialogOpen(false)
            onChanged()
          }}
        />
      )}
    </Box>
  )
}

export default PayloadStreamsWorkspace
