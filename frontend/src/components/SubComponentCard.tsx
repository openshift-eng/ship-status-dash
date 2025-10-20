import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import React, { useState, useEffect } from 'react'

import { StatusChip } from './StatusColors'
import type { SubComponent } from '../types'
import OutageModal from './OutageModal'
import { getSubComponentStatusEndpoint } from '../utils/endpoints'
import { getStatusChipColor, getStatusBackgroundColor } from '../utils/helpers'

const SubComponentCard = styled(Card)<{ status: string; useBackgroundColor?: boolean }>(({
  theme,
  status,
  useBackgroundColor = false,
}) => {
  const color = getStatusChipColor(theme, status)
  const backgroundColor = useBackgroundColor
    ? getStatusBackgroundColor(theme, status)
    : theme.palette.background.paper

  return {
    border: `2px solid ${color}`,
    borderRadius: theme.spacing(1.5),
    cursor: 'pointer',
    transition: 'all 0.2s ease-in-out',
    backgroundColor: backgroundColor,
    minHeight: '120px',
    display: 'flex',
    flexDirection: 'column',
    '&:hover': {
      boxShadow: theme.shadows[4],
      transform: 'translateY(-1px)',
      '& .MuiChip-root': {
        color: 'white',
        borderColor: 'white',
      },
    },
  }
})

const StyledCardContent = styled(CardContent)(({ theme }) => ({
  padding: theme.spacing(2.5),
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  '&:last-child': {
    paddingBottom: theme.spacing(2.5),
  },
}))

const CardHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: theme.spacing(1),
}))

const SubComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1rem',
  color: theme.palette.text.primary,
  flex: 1,
  marginRight: theme.spacing(1),
}))

const SubComponentDescription = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.secondary,
  lineHeight: 1.5,
  flex: 1,
}))

const StatusChipBox = styled(Box)(() => ({
  flexShrink: 0,
}))

interface SubComponentCardProps {
  subComponent: SubComponent
  componentName: string
  useBackgroundColor?: boolean
}

const SubComponentCardComponent: React.FC<SubComponentCardProps> = ({
  subComponent,
  componentName,
  useBackgroundColor = false,
}) => {
  const [modalOpen, setModalOpen] = useState(false)
  const [subComponentWithStatus, setSubComponentWithStatus] = useState<SubComponent>(subComponent)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(getSubComponentStatusEndpoint(componentName, subComponent.name))
      .then((res) => res.json())
      .catch(() => ({ status: 'Unknown', active_outages: [] }))
      .then((subStatus) => {
        setSubComponentWithStatus({
          ...subComponent,
          status: subStatus.status,
          active_outages: subStatus.active_outages,
        })
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponent])

  const handleClick = () => {
    setModalOpen(true)
  }

  const handleCloseModal = () => {
    setModalOpen(false)
  }

  return (
    <>
      <SubComponentCard
        status={subComponentWithStatus.status || 'Unknown'}
        useBackgroundColor={useBackgroundColor}
        onClick={handleClick}
      >
        <StyledCardContent>
          <CardHeader>
            <SubComponentTitle>{subComponent.name}</SubComponentTitle>
            <StatusChipBox>
              <StatusChip
                label={loading ? 'Loading...' : subComponentWithStatus.status || 'Unknown'}
                status={subComponentWithStatus.status || 'Unknown'}
                size="small"
                variant="outlined"
              />
            </StatusChipBox>
          </CardHeader>
          <SubComponentDescription>{subComponent.description}</SubComponentDescription>
        </StyledCardContent>
      </SubComponentCard>

      <OutageModal
        open={modalOpen}
        onClose={handleCloseModal}
        selectedSubComponent={subComponentWithStatus}
        componentName={componentName}
      />
    </>
  )
}

export default SubComponentCardComponent
