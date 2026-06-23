import { Alert, Button, Typography } from '@mui/material'

import { useAuth } from '../../contexts/AuthContext'
import type { SuspectedOutageInfo } from '../../types'
import { relativeTime } from '../../utils/helpers'

interface SuspectedReportsBannerProps {
  suspected: SuspectedOutageInfo
  componentSlug: string
  subComponentName: string
  hasUserReported: boolean
  onReportClick: () => void
}

const SuspectedReportsBanner = ({
  suspected,
  componentSlug,
  subComponentName,
  hasUserReported,
  onReportClick,
}: SuspectedReportsBannerProps) => {
  const { user, isComponentAdmin } = useAuth()
  const showReportButton = !!user && !isComponentAdmin(componentSlug) && !hasUserReported

  const reportCount = suspected.report_count
  const reportLabel = reportCount === 1 ? 'report' : 'reports'
  const timeAgo = relativeTime(new Date(suspected.start_time), new Date())

  return (
    <Alert
      severity="warning"
      icon={false}
      sx={{ mb: 2, py: 2, px: 3 }}
      action={
        showReportButton ? (
          <Button variant="outlined" size="small" color="warning" onClick={onReportClick}>
            Experiencing this?
          </Button>
        ) : undefined
      }
    >
      <Typography variant="subtitle1" fontWeight={600}>
        Users are reporting an issue with {subComponentName}
      </Typography>
      {suspected.description && (
        <Typography variant="body2" fontStyle="italic">
          &ldquo;{suspected.description}&rdquo;
        </Typography>
      )}
      <Typography variant="body2" sx={{ opacity: 0.8 }}>
        {reportCount} {reportLabel} &middot; {timeAgo}
      </Typography>
    </Alert>
  )
}

export default SuspectedReportsBanner
