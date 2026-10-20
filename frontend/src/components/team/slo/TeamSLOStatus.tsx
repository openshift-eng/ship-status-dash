import { Box, Chip, Divider, styled, Tooltip, Typography } from '@mui/material'

import type { SLOComponentBlock, SLOEvaluation } from '../../../types'

import { controllerDomId, streamDomId } from './format'
import {
  groupStreamsByController,
  streamTooltip,
  trtPayloadResult,
  worstMiss,
} from './trt/v1/result'

const Section = styled(Box)(({ theme }) => ({
  backgroundColor: theme.palette.background.paper,
  borderRadius: theme.spacing(2),
  padding: theme.spacing(3),
  marginBottom: theme.spacing(3),
  border: `1px solid ${theme.palette.divider}`,
}))

const Metrics = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(3),
}))

const MetricLabel = styled(Typography)(({ theme }) => ({
  fontSize: '0.8rem',
  color: theme.palette.text.secondary,
}))

const SectionTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1.25rem',
  marginBottom: theme.spacing(2),
}))

const MetricValue = styled(Typography)({
  fontSize: '1.5rem',
  fontWeight: 600,
})

const WorstMissValue = styled(MetricValue)({
  fontSize: '1.1rem',
})

const ChipRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(1),
}))

const EvalRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  gap: theme.spacing(4),
  alignItems: 'flex-start',
}))

const StreamChipsPanel = styled(Box)(({ theme }) => ({
  flex: 1,
  minWidth: 0,
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(1),
}))

const StreamChip = styled(Chip)({
  fontSize: '0.8125rem',
})

const ControllerSection = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(1),
}))

const ControllerChip = styled(Chip)(({ theme }) => ({
  borderRadius: theme.spacing(0.5),
  height: 28,
  fontSize: '0.75rem',
  fontWeight: 600,
  marginBottom: theme.spacing(0.5),
}))

interface TeamSLOStatusProps {
  team: string
  evaluations: SLOEvaluation[]
  sloComponents: SLOComponentBlock[]
}

const TeamSLOStatus = ({ team, evaluations, sloComponents }: TeamSLOStatusProps) => {
  const incidentCount = sloComponents.reduce((sum, block) => sum + block.outages.length, 0)

  return (
    <Section id="slo">
      <SectionTitle>{team} SLOs</SectionTitle>
      {evaluations.map((evaluation) => {
        const result = trtPayloadResult(evaluation)
        const streamEvals = result ? result.streams : []
        const metCount = streamEvals.filter((stream) => stream.met).length
        const miss = result ? worstMiss(result) : undefined
        const controllerGroups = groupStreamsByController(streamEvals)
        const multipleControllers = controllerGroups.length > 1
        return (
          <EvalRow key={evaluation.name}>
            <Metrics>
              <Box>
                <MetricLabel>{evaluation.display_name || evaluation.name}</MetricLabel>
                {result && (
                  <MetricValue>
                    {metCount} of {streamEvals.length}
                  </MetricValue>
                )}
                <Chip
                  size="small"
                  label={evaluation.met ? 'Met' : 'Missed'}
                  color={evaluation.met ? 'success' : 'warning'}
                />
              </Box>
              {miss && (
                <Box>
                  <MetricLabel>Worst miss</MetricLabel>
                  <WorstMissValue>{miss.key}</WorstMissValue>
                  <MetricLabel>
                    {miss.last_accepted_at
                      ? `Last accepted ${formatAge(miss.last_accepted_at)} ago`
                      : 'No accepted payload stored'}
                  </MetricLabel>
                </Box>
              )}
              {incidentCount > 0 && (
                <Box>
                  <MetricLabel>Open incidents</MetricLabel>
                  <MetricValue>{incidentCount}</MetricValue>
                </Box>
              )}
            </Metrics>
            <StreamChipsPanel>
              {controllerGroups.map(({ controller, streams }, groupIndex) => (
                <ControllerSection key={`${controller}-${groupIndex}`}>
                  {groupIndex > 0 && <Divider sx={{ mb: 1 }} />}
                  {multipleControllers && (
                    <ControllerChip
                      label={controller}
                      size="small"
                      variant="outlined"
                      clickable
                      component="a"
                      href={`#${controllerDomId(controller)}`}
                    />
                  )}
                  <ChipRow>
                    {streams.map((stream) => (
                      <Tooltip
                        key={stream.key}
                        title={streamTooltip(stream, result?.window || '')}
                        describeChild
                        arrow
                      >
                        <StreamChip
                          label={stream.key}
                          aria-label={`${stream.key}: SLO ${stream.met ? 'met' : 'missed'}`}
                          color={stream.met ? 'success' : 'warning'}
                          clickable
                          component="a"
                          href={`#${streamDomId(stream.key)}`}
                        />
                      </Tooltip>
                    ))}
                  </ChipRow>
                </ControllerSection>
              ))}
            </StreamChipsPanel>
          </EvalRow>
        )
      })}
    </Section>
  )
}

export default TeamSLOStatus
