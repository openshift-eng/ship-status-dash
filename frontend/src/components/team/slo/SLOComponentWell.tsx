import HistoryIcon from '@mui/icons-material/History'
import { Box, Card, IconButton, styled, Typography } from '@mui/material'
import { useNavigate } from 'react-router'

import type { SLOComponentBlock } from '../../../types'
import { worstOutageStatus } from '../../../utils/helpers'
import { getStatusTintStyles } from '../../../utils/styles'

import { sloComponentDomId, subComponentPath } from './format'
import SLOOutageCard from './SLOOutageCard'

const Section = styled(Card)<{ status?: string }>(({ theme, status }) => ({
  ...(status ? getStatusTintStyles(theme, status, 2) : {}),
  borderRadius: theme.spacing(2),
  padding: theme.spacing(3),
  marginBottom: theme.spacing(3),
}))

const TitleRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(2),
}))

const Title = styled(Typography)(() => ({
  fontWeight: 600,
  fontSize: '1.25rem',
}))

const OutageList = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 280px))',
  gap: theme.spacing(2),
  justifyContent: 'start',
}))

const Meta = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.875rem',
}))

interface SLOComponentWellProps {
  block: SLOComponentBlock
}

const SLOComponentWell = ({ block }: SLOComponentWellProps) => {
  const navigate = useNavigate()

  return (
    <Section
      id={sloComponentDomId(block.component, block.sub_component)}
      status={worstOutageStatus(block.outages)}
    >
      <TitleRow>
        <Title>
          {block.component} / {block.sub_component}
        </Title>
        <IconButton
          size="small"
          aria-label={`View ${block.sub_component} outage history`}
          onClick={() => navigate(subComponentPath(block.component, block.sub_component))}
        >
          <HistoryIcon fontSize="small" />
        </IconButton>
      </TitleRow>
      {block.outages.length === 0 && <Meta>No active outages</Meta>}
      {block.outages.length > 0 && (
        <OutageList>
          {block.outages.map((outage) => (
            <SLOOutageCard key={outage.ID} outage={outage} showJira />
          ))}
        </OutageList>
      )}
    </Section>
  )
}

export default SLOComponentWell
