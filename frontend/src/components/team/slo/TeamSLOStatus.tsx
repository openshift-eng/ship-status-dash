import { Box, Chip, styled, Tooltip, Typography } from '@mui/material'

import type { SLOComponentBlock, SLOEvaluation } from '../../../types'

import { formatAge, streamDomId } from './format'
import { groupStreamsByController, parseHours, trtPayloadResult, worstMiss } from './trt/v1/result'

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
  marginBottom: theme.spacing(2),
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

const ControllerSection = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(1.5),
}))

const ControllerLabel = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(0.75),
}))

const streamTooltip = (
  stream: { last_accepted_at?: string; window?: string },
  defaultWindow: string,
) => {
  const age = stream.last_accepted_at ? formatAge(stream.last_accepted_at) : 'never'
  const windowHours = parseHours(stream.window || defaultWindow)
  return `Last accepted ${age} ago; expected every ${windowHours}h`
}

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
          <Box key={evaluation.name}>
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
            {controllerGroups.map(({ controller, url, streams }) => (
              <ControllerSection key={controller}>
                {multipleControllers && (
                  <ControllerLabel>
                    <Chip
                      size="small"
                      variant="outlined"
                      label={controller}
                      clickable={!!url}
                      component={url ? 'a' : 'span'}
                      {...(url ? { href: url, target: '_blank', rel: 'noopener noreferrer' } : {})}
                    />
                  </ControllerLabel>
                )}
                <ChipRow>
                  {streams.map((stream) => (
                    <Tooltip
                      key={stream.key}
                      title={streamTooltip(stream, result?.window || '')}
                      arrow
                    >
                      <Chip
                        size="small"
                        label={stream.key}
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
          </Box>
        )
      })}
    </Section>
  )
}

export default TeamSLOStatus
