import { Box, Button, Paper, styled, Typography } from '@mui/material'

import { useAuth } from '../../contexts/AuthContext'
import type { SuspectedOutageInfo } from '../../types'
import { relativeTime } from '../../utils/helpers'
import { getStatusTintStyles } from '../../utils/styles'

const BannerPaper = styled(Paper)(({ theme }) => ({
  ...getStatusTintStyles(theme, 'Suspected', 'inherit'),
  padding: theme.spacing(2, 3),
  marginBottom: theme.spacing(2),
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  gap: theme.spacing(2),
}))

const MetaText = styled(Typography)({
  opacity: 0.8,
})

interface SuspectedReportsBannerProps {
  suspected: SuspectedOutageInfo
  componentSlug: string
  subComponentName: string
  onReportClick: () => void
}

const SuspectedReportsBanner = ({
  suspected,
  componentSlug,
  subComponentName,
  onReportClick,
}: SuspectedReportsBannerProps) => {
  const { user, isComponentAdmin } = useAuth()
  const showReportButton = !!user && !isComponentAdmin(componentSlug)

  const reportCount = suspected.report_count
  const reportLabel = reportCount === 1 ? 'report' : 'reports'
  const timeAgo = relativeTime(new Date(suspected.start_time), new Date())

  return (
    <BannerPaper elevation={0}>
      <Box>
        <Typography variant="subtitle1" fontWeight={600}>
          Users are reporting an issue with {subComponentName}
        </Typography>
        {suspected.description && (
          <Typography variant="body2" color="text.secondary" fontStyle="italic">
            &ldquo;{suspected.description}&rdquo;
          </Typography>
        )}
        <MetaText variant="body2">
          {reportCount} {reportLabel} &middot; {timeAgo}
        </MetaText>
      </Box>
      {showReportButton && (
        <Button variant="outlined" size="small" onClick={onReportClick}>
          Experiencing this?
        </Button>
      )}
    </BannerPaper>
  )
}

export default SuspectedReportsBanner
