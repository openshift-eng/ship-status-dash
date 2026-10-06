import ExpandMore from '@mui/icons-material/ExpandMore'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  styled,
  TextField,
} from '@mui/material'
import { useState } from 'react'
import type { ReactNode, SyntheticEvent } from 'react'

import type {
  SLOItem,
  SLOItemLink,
  SLOJob,
  SLOSharedCause,
  SLOSharedCauseLink,
} from '../../../../../types'
import {
  deleteSLOItemLinkEndpoint,
  putSLOItemEndpoint,
  putSLOItemLinkEndpoint,
} from '../../../../../utils/endpoints'
import { formatDateForDateTimeLocal } from '../../../../../utils/helpers'

const PAYLOAD_STREAMS_KIND = 'payload_streams'
const SCHEMA_VERSION = 1

const Content = styled(DialogContent)(({ theme }) => ({
  paddingTop: theme.spacing(1),
}))

const Field = styled(TextField)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const FormAccordion = styled(Accordion)(({ theme }) => ({
  borderRadius: theme.spacing(1),
  '&:before': {
    display: 'none',
  },
  '&.Mui-expanded': {
    margin: 0,
  },
  '&:first-of-type, &:last-of-type': {
    borderRadius: theme.spacing(1),
  },
}))

const Summary = styled(AccordionSummary)(({ theme }) => ({
  fontWeight: 600,
  '& .MuiAccordionSummary-content, & .MuiAccordionSummary-content.Mui-expanded': {
    margin: theme.spacing(1.5, 0),
  },
}))

const SummaryContent = styled('span')(({ theme }) => ({
  display: 'flex',
  alignItems: 'baseline',
  justifyContent: 'space-between',
  width: '100%',
  gap: theme.spacing(2),
}))

const SummaryCount = styled('span')(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.875rem',
  fontWeight: 400,
}))

const Details = styled(AccordionDetails)({
  paddingTop: 0,
  display: 'flex',
  flexDirection: 'column',
})

const Entry = styled('div')(({ theme }) => ({
  border: `1px solid ${theme.palette.divider}`,
  borderRadius: theme.spacing(1),
  padding: theme.spacing(2),
  marginBottom: theme.spacing(2),
  backgroundColor: theme.palette.background.default,
}))

const EntryActions = styled('div')({
  display: 'flex',
  justifyContent: 'flex-end',
})

const ErrorText = styled('p')(({ theme }) => ({
  color: theme.palette.error.main,
  margin: theme.spacing(1, 0),
}))

const Fields = styled('fieldset')(({ theme }) => ({
  border: 0,
  margin: 0,
  padding: 0,
  minWidth: 0,
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(1),
  '&:disabled': {
    pointerEvents: 'none',
  },
}))

interface JobDraft {
  draftId: string
  name: string
  url: string
  state: string
  notes: string
  noteIds: string[]
  laterPassTag: string
  laterPassURL: string
  recurring_count?: number
}

interface CauseLinkDraft {
  draftId: string
  label: string
  url: string
}

interface NoteDraft {
  draftId: string
  id: string
  text: string
  url: string
  links: CauseLinkDraft[]
}

interface LinkDraft {
  id?: number
  url: string
  link_type: SLOItemLink['link_type']
}

interface UpsertPayloadItemDialogProps {
  open: boolean
  team: string
  streams: string[]
  item?: SLOItem
  onClose: () => void
  onSuccess: () => void
}

let jobDraftSeq = 0
let noteDraftSeq = 0
let causeLinkSeq = 0

const newCauseLink = (partial?: Partial<SLOSharedCauseLink>): CauseLinkDraft => {
  causeLinkSeq += 1
  return {
    draftId: `cause-link-${causeLinkSeq}`,
    label: partial?.label ?? '',
    url: partial?.url ?? '',
  }
}

const newJobDraft = (
  partial?: Partial<JobDraft> & { note_ids?: string[]; later_pass?: { tag: string; url: string } },
): JobDraft => {
  jobDraftSeq += 1
  return {
    draftId: `job-${jobDraftSeq}`,
    name: partial?.name ?? '',
    url: partial?.url ?? '',
    state: partial?.state?.trim() || 'failure',
    notes: partial?.notes ?? '',
    noteIds: partial?.noteIds ?? partial?.note_ids ?? [],
    laterPassTag: partial?.laterPassTag ?? partial?.later_pass?.tag ?? '',
    laterPassURL: partial?.laterPassURL ?? partial?.later_pass?.url ?? '',
    recurring_count: partial?.recurring_count,
  }
}

const migrateLegacyPasses = (notes: SLOSharedCause[], jobs: SLOJob[]) => {
  const converted = new Set<string>()
  const nextJobs = jobs.map((job) => {
    if (job.later_pass) {
      return job
    }
    const prefix = `passed:${job.name}:`
    const note = notes.find(
      (item) => !converted.has(item.id) && item.id.startsWith(prefix) && item.url,
    )
    if (!note?.url) {
      return job
    }
    converted.add(note.id)
    return { ...job, later_pass: { tag: note.id.slice(prefix.length), url: note.url } }
  })
  return {
    notes: notes.filter((note) => !converted.has(note.id)),
    jobs: nextJobs.map((job) => ({
      ...job,
      note_ids: (job.note_ids ?? []).filter((id) => !converted.has(id)),
    })),
  }
}

const newNoteDraft = (
  partial?: Partial<Omit<NoteDraft, 'links'>> & { links?: SLOSharedCauseLink[] },
): NoteDraft => {
  noteDraftSeq += 1
  return {
    draftId: `note-${noteDraftSeq}`,
    id: partial?.id ?? '',
    text: partial?.text ?? '',
    url: partial?.url ?? '',
    links: (partial?.links ?? []).map((link) => newCauseLink(link)),
  }
}

const causeChoices = (notes: NoteDraft[], selected: string[]) => {
  const ids = notes
    .map((note) => note.id.trim())
    .filter((id) => id !== '' && !id.startsWith('passed:'))
  selected.forEach((id) => {
    if (id !== '' && !ids.includes(id)) {
      ids.push(id)
    }
  })
  return ids
}

const emptyLink = (): LinkDraft => ({ url: '', link_type: 'jira' })

interface LinkPair {
  index: number
  draft: LinkDraft
  keep?: SLOItemLink
}

const pairDesiredLinks = (drafts: LinkDraft[], baseline: SLOItemLink[]): LinkPair[] => {
  const used = new Set<number>()
  const pairs: LinkPair[] = []
  drafts.forEach((draft, index) => {
    const url = draft.url.trim()
    if (url === '') {
      return
    }
    const desired: LinkDraft = { ...draft, url }
    const byId =
      desired.id != null
        ? baseline.find((item) => item.ID === desired.id && !used.has(item.ID))
        : undefined
    if (byId && byId.url === desired.url && byId.link_type === desired.link_type) {
      used.add(byId.ID)
      pairs.push({ index, draft: desired, keep: byId })
      return
    }
    const byValue = baseline.find(
      (item) =>
        !used.has(item.ID) &&
        item.ID !== byId?.ID &&
        item.url === desired.url &&
        item.link_type === desired.link_type,
    )
    if (byValue) {
      used.add(byValue.ID)
      pairs.push({ index, draft: desired, keep: byValue })
      return
    }
    pairs.push({ index, draft: desired })
  })
  return pairs
}

type SectionKey = 'payload' | 'causes' | 'jobs' | 'links'

const SectionAccordion = ({
  title,
  count,
  expanded,
  onChange,
  children,
}: {
  title: string
  count?: number
  expanded: boolean
  onChange: (event: SyntheticEvent, expanded: boolean) => void
  children: ReactNode
}) => (
  <FormAccordion disableGutters variant="outlined" expanded={expanded} onChange={onChange}>
    <Summary expandIcon={<ExpandMore />}>
      <SummaryContent>
        {title}
        {count !== undefined && <SummaryCount>{count}</SummaryCount>}
      </SummaryContent>
    </Summary>
    <Details>{children}</Details>
  </FormAccordion>
)

const initialLinks = (item?: SLOItem): LinkDraft[] => {
  const existing = (item ? item.links : []).map((link) => ({
    id: link.ID,
    url: link.url,
    link_type: link.link_type,
  }))
  return existing.length > 0 ? existing : [emptyLink()]
}

const UpsertPayloadItemDialog = ({
  open,
  team,
  streams,
  item,
  onClose,
  onSuccess,
}: UpsertPayloadItemDialogProps) => {
  const editing = !!item
  const [stream, setStream] = useState(item?.group_key ?? streams[0] ?? '')
  const [tag, setTag] = useState(item?.item_key ?? '')
  const [occurredAt, setOccurredAt] = useState(
    item
      ? formatDateForDateTimeLocal(new Date(item.occurred_at))
      : formatDateForDateTimeLocal(new Date()),
  )
  const [outcome, setOutcome] = useState(item?.outcome ?? 'Rejected')
  const [payloadURL, setPayloadURL] = useState(item?.details.payload_url ?? '')
  const [analysisURL, setAnalysisURL] = useState(item?.details.analysis_url ?? '')
  const [notes, setNotes] = useState(item?.notes ?? '')
  const migrated = migrateLegacyPasses(item?.details.shared_causes ?? [], item?.details.jobs ?? [])
  const [payloadNotes, setPayloadNotes] = useState<NoteDraft[]>(
    migrated.notes.map((note) => newNoteDraft(note)),
  )
  const [jobs, setJobs] = useState<JobDraft[]>(migrated.jobs.map((job) => newJobDraft(job)))
  const [links, setLinks] = useState<LinkDraft[]>(initialLinks(item))
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [expanded, setExpanded] = useState<Record<SectionKey, boolean>>({
    payload: true,
    causes: false,
    jobs: false,
    links: false,
  })

  const toggleSection = (key: SectionKey) => (_event: SyntheticEvent, isExpanded: boolean) => {
    setExpanded((current) => ({ ...current, [key]: isExpanded }))
  }

  const updateJob = (index: number, patch: Partial<JobDraft>) => {
    setJobs((current) => current.map((job, i) => (i === index ? { ...job, ...patch } : job)))
  }

  const updatePayloadNote = (index: number, patch: Partial<NoteDraft>) => {
    setPayloadNotes((current) =>
      current.map((note, i) => (i === index ? { ...note, ...patch } : note)),
    )
  }

  const updateLink = (index: number, patch: Partial<LinkDraft>) => {
    setLinks((current) => current.map((link, i) => (i === index ? { ...link, ...patch } : link)))
  }

  const readError = async (response: Response, fallback: string) => {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null
    return payload?.error || fallback
  }

  const syncLinks = async (saved: SLOItem) => {
    let baseline = [...(saved.links ?? [])]
    const pairs = pairDesiredLinks(links, baseline)
    const nextLinks = links.map((link) => ({ ...link }))
    for (const pair of pairs) {
      if (!pair.keep) {
        continue
      }
      nextLinks[pair.index] = {
        ...nextLinks[pair.index],
        id: pair.keep.ID,
        url: pair.draft.url,
        link_type: pair.draft.link_type,
      }
    }

    const keptIds = new Set(pairs.flatMap((pair) => (pair.keep ? [pair.keep.ID] : [])))
    for (const existing of editing ? baseline : []) {
      if (keptIds.has(existing.ID)) {
        continue
      }
      const response = await fetch(
        deleteSLOItemLinkEndpoint(team, saved.kind, saved.item_key, existing.ID),
        { method: 'DELETE', credentials: 'include' },
      )
      if (!response.ok && response.status !== 404) {
        setLinks(nextLinks)
        setError(await readError(response, `Link delete failed (${response.status})`))
        return false
      }
      baseline = baseline.filter((link) => link.ID !== existing.ID)
    }

    for (const pair of pairs) {
      if (pair.keep) {
        continue
      }
      const response = await fetch(putSLOItemLinkEndpoint(team, saved.kind, saved.item_key), {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: pair.draft.url, link_type: pair.draft.link_type }),
      })
      if (!response.ok) {
        setLinks(nextLinks)
        setError(await readError(response, `Link failed (${response.status})`))
        return false
      }
      const created = (await response.json()) as SLOItemLink
      nextLinks[pair.index] = {
        ...nextLinks[pair.index],
        id: created.ID,
        url: pair.draft.url,
        link_type: pair.draft.link_type,
      }
      baseline = [...baseline, created]
    }
    setLinks(nextLinks)
    return true
  }

  const submit = async () => {
    const occurred = new Date(occurredAt)
    if (occurredAt.trim() === '' || Number.isNaN(occurred.getTime())) {
      setError('Occurred at must be a valid time')
      return
    }
    setSaving(true)
    setError('')
    let succeeded = false
    try {
      const bodyNotes: SLOSharedCause[] = payloadNotes
        .filter((note) => note.id.trim() !== '' || note.text.trim() !== '')
        .map((note) => {
          const noteLinks = note.links
            .filter((link) => link.label.trim() !== '' || link.url.trim() !== '')
            .map((link) => ({ label: link.label.trim(), url: link.url.trim() }))
          return {
            id: note.id.trim(),
            text: note.text.trim(),
            ...(note.url.trim() ? { url: note.url.trim() } : {}),
            ...(noteLinks.length > 0 ? { links: noteLinks } : {}),
          }
        })
      const retainedCauseIDs = new Set(bodyNotes.map((cause) => cause.id))
      const bodyJobs: SLOJob[] = jobs
        .filter((job) => job.name.trim() !== '')
        .map((job) => {
          const noteIDs = job.noteIds
            .map((id) => id.trim())
            .filter((id) => id !== '' && (retainedCauseIDs.has(id) || !id.startsWith('passed:')))
          const laterTag = job.laterPassTag.trim()
          const laterURL = job.laterPassURL.trim()
          return {
            name: job.name.trim(),
            url: job.url.trim(),
            state: job.state.trim() || 'failure',
            notes: job.notes,
            ...(noteIDs.length > 0 ? { note_ids: noteIDs } : {}),
            ...(laterTag !== '' && laterURL !== ''
              ? { later_pass: { tag: laterTag, url: laterURL } }
              : {}),
            ...(job.recurring_count !== undefined ? { recurring_count: job.recurring_count } : {}),
          }
        })
      const response = await fetch(putSLOItemEndpoint(team), {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          kind: PAYLOAD_STREAMS_KIND,
          schema_version: SCHEMA_VERSION,
          item_key: tag.trim(),
          group_key: stream,
          occurred_at:
            item && occurredAt === formatDateForDateTimeLocal(new Date(item.occurred_at))
              ? item.occurred_at
              : occurred.toISOString(),
          outcome,
          notes,
          details: {
            payload_url: payloadURL.trim(),
            ...(analysisURL.trim() ? { analysis_url: analysisURL.trim() } : {}),
            ...(item?.details.finished_at ? { finished_at: item.details.finished_at } : {}),
            ...(bodyNotes.length > 0 ? { shared_causes: bodyNotes } : {}),
            jobs: bodyJobs,
          },
        }),
      })
      if (!response.ok) {
        setError(await readError(response, `Save failed (${response.status})`))
        return
      }
      const saved = (await response.json()) as SLOItem
      if (!(await syncLinks(saved))) {
        return
      }
      succeeded = true
    } catch {
      setError('Save failed')
    } finally {
      setSaving(false)
    }
    if (succeeded) {
      onSuccess()
    }
  }

  const handleClose = () => {
    if (!saving) {
      onClose()
    }
  }

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="lg">
      <DialogTitle>{editing ? 'Edit payload' : 'Add payload'}</DialogTitle>
      <Content>
        <Fields disabled={saving}>
          <SectionAccordion
            title="Payload"
            expanded={expanded.payload}
            onChange={toggleSection('payload')}
          >
            <Field
              fullWidth
              multiline
              minRows={2}
              label="Payload notes"
              value={notes}
              helperText="Freeform note shown with the payload details"
              onChange={(event) => setNotes(event.target.value)}
            />
            <Field
              select
              fullWidth
              label="Stream"
              value={stream}
              disabled={editing}
              onChange={(event) => setStream(event.target.value)}
            >
              {streams.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </Field>
            <Field
              fullWidth
              label="Tag"
              value={tag}
              disabled={editing}
              onChange={(event) => setTag(event.target.value)}
            />
            <Field
              fullWidth
              label="Occurred at"
              type="datetime-local"
              value={occurredAt}
              onChange={(event) => setOccurredAt(event.target.value)}
              slotProps={{ inputLabel: { shrink: true } }}
            />
            <Field
              select
              fullWidth
              label="Phase"
              value={outcome}
              onChange={(event) => setOutcome(event.target.value)}
            >
              {['Accepted', 'Rejected', 'Ready'].map((value) => (
                <MenuItem key={value} value={value}>
                  {value}
                </MenuItem>
              ))}
            </Field>
            <Field
              fullWidth
              label="Release controller URL"
              value={payloadURL}
              onChange={(event) => setPayloadURL(event.target.value)}
            />
            <Field
              fullWidth
              label="Payload agent URL"
              value={analysisURL}
              onChange={(event) => setAnalysisURL(event.target.value)}
            />
          </SectionAccordion>
          <SectionAccordion
            title="Shared root causes"
            count={payloadNotes.length}
            expanded={expanded.causes}
            onChange={toggleSection('causes')}
          >
            {payloadNotes.map((note, index) => (
              <Entry key={note.draftId}>
                <Field
                  fullWidth
                  label="Cause id"
                  value={note.id}
                  onChange={(event) => updatePayloadNote(index, { id: event.target.value })}
                />
                <Field
                  fullWidth
                  label="Title"
                  value={note.text}
                  onChange={(event) => updatePayloadNote(index, { text: event.target.value })}
                />
                {note.links.map((link, linkIndex) => (
                  <Box key={link.draftId}>
                    <Field
                      fullWidth
                      label="Link label"
                      value={link.label}
                      onChange={(event) =>
                        updatePayloadNote(index, {
                          links: note.links.map((item, i) =>
                            i === linkIndex ? { ...item, label: event.target.value } : item,
                          ),
                        })
                      }
                    />
                    <Field
                      fullWidth
                      label="Link URL"
                      value={link.url}
                      onChange={(event) =>
                        updatePayloadNote(index, {
                          links: note.links.map((item, i) =>
                            i === linkIndex ? { ...item, url: event.target.value } : item,
                          ),
                        })
                      }
                    />
                    <EntryActions>
                      <Button
                        variant="outlined"
                        color="error"
                        onClick={() =>
                          updatePayloadNote(index, {
                            links: note.links.filter((_, i) => i !== linkIndex),
                          })
                        }
                      >
                        Remove link
                      </Button>
                    </EntryActions>
                  </Box>
                ))}
                <Button
                  variant="outlined"
                  onClick={() =>
                    updatePayloadNote(index, { links: [...note.links, newCauseLink()] })
                  }
                >
                  Add link
                </Button>
                <Field
                  fullWidth
                  label="Link URL"
                  value={note.url}
                  helperText="Optional single link. Use the list above for more than one."
                  onChange={(event) => updatePayloadNote(index, { url: event.target.value })}
                />
                <EntryActions>
                  <Button
                    variant="outlined"
                    color="error"
                    onClick={() => {
                      const removedID = note.id.trim()
                      setPayloadNotes((current) => current.filter((_, i) => i !== index))
                      if (removedID === '') {
                        return
                      }
                      setJobs((current) =>
                        current.map((job) => ({
                          ...job,
                          noteIds: job.noteIds.filter((id) => id.trim() !== removedID),
                        })),
                      )
                    }}
                  >
                    Remove cause
                  </Button>
                </EntryActions>
              </Entry>
            ))}
            <Button
              variant="outlined"
              color="primary"
              onClick={() => setPayloadNotes((current) => [...current, newNoteDraft()])}
            >
              Add cause
            </Button>
          </SectionAccordion>
          <SectionAccordion
            title="Failed jobs"
            count={jobs.length}
            expanded={expanded.jobs}
            onChange={toggleSection('jobs')}
          >
            {jobs.map((job, index) => (
              <Entry key={job.draftId}>
                <Field
                  fullWidth
                  label="Job name"
                  value={job.name}
                  onChange={(event) => updateJob(index, { name: event.target.value })}
                />
                <Field
                  fullWidth
                  label="Job URL"
                  value={job.url}
                  onChange={(event) => updateJob(index, { url: event.target.value })}
                />
                <Field
                  select
                  fullWidth
                  label="Root causes"
                  value={job.noteIds}
                  helperText="Choose one or more shared causes"
                  slotProps={{ select: { multiple: true } }}
                  onChange={(event) => {
                    const value = event.target.value as string | string[]
                    updateJob(index, {
                      noteIds: typeof value === 'string' ? value.split(',') : value,
                    })
                  }}
                >
                  {causeChoices(payloadNotes, job.noteIds).map((id) => (
                    <MenuItem key={id} value={id}>
                      {id}
                    </MenuItem>
                  ))}
                </Field>
                <Field
                  fullWidth
                  label="Job-specific note"
                  value={job.notes}
                  onChange={(event) => updateJob(index, { notes: event.target.value })}
                />
                <Field
                  fullWidth
                  label="Later pass tag"
                  value={job.laterPassTag}
                  helperText="Newer payload where this job succeeded"
                  onChange={(event) => updateJob(index, { laterPassTag: event.target.value })}
                />
                <Field
                  fullWidth
                  label="Later pass URL"
                  value={job.laterPassURL}
                  onChange={(event) => updateJob(index, { laterPassURL: event.target.value })}
                />
                <EntryActions>
                  <Button
                    variant="outlined"
                    color="error"
                    onClick={() => setJobs((current) => current.filter((_, i) => i !== index))}
                  >
                    Remove job
                  </Button>
                </EntryActions>
              </Entry>
            ))}
            <Button
              variant="outlined"
              color="primary"
              onClick={() => setJobs((current) => [...current, newJobDraft()])}
            >
              Add job
            </Button>
          </SectionAccordion>
          <SectionAccordion
            title="Links"
            count={links.filter((link) => link.url.trim() !== '').length}
            expanded={expanded.links}
            onChange={toggleSection('links')}
          >
            {links.map((link, index) => (
              <Entry key={link.id ?? `new-${index}`}>
                <Field
                  fullWidth
                  label="Link URL"
                  value={link.url}
                  onChange={(event) => updateLink(index, { url: event.target.value })}
                />
                <Field
                  select
                  fullWidth
                  label="Link type"
                  value={link.link_type}
                  onChange={(event) =>
                    updateLink(index, { link_type: event.target.value as LinkDraft['link_type'] })
                  }
                >
                  {(['jira', 'outage', 'other'] as const).map((value) => (
                    <MenuItem key={value} value={value}>
                      {value}
                    </MenuItem>
                  ))}
                </Field>
                <EntryActions>
                  <Button
                    variant="outlined"
                    color="error"
                    onClick={() => setLinks((current) => current.filter((_, i) => i !== index))}
                  >
                    Remove link
                  </Button>
                </EntryActions>
              </Entry>
            ))}
            <Button
              variant="outlined"
              color="primary"
              onClick={() => setLinks((current) => [...current, emptyLink()])}
            >
              Add link
            </Button>
          </SectionAccordion>
        </Fields>
        {error && <ErrorText>{error}</ErrorText>}
      </Content>
      <DialogActions>
        <Button onClick={handleClose} disabled={saving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={saving || !tag.trim() || !payloadURL.trim() || occurredAt.trim() === ''}
          onClick={() => void submit()}
        >
          Save
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default UpsertPayloadItemDialog
