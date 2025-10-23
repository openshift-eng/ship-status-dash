import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import React, { useEffect, useState } from 'react'

import type { SubComponent } from '../../types'
import { getComponentInfoEndpoint, getSubComponentStatusEndpoint } from '../../utils/endpoints'
import { getStatusBackgroundColor, getStatusChipColor } from '../../utils/helpers'
import OutageModal from '../outage/OutageModal'
import { StatusChip } from '../StatusColors'

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
        backgroundColor: 'white',
        color: color,
        borderColor: color,
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
  const [requiresConfirmation, setRequiresConfirmation] = useState<boolean>(false)

  useEffect(() => {
    Promise.all([
      fetch(getSubComponentStatusEndpoint(componentName, subComponent.name)),
      fetch(getComponentInfoEndpoint(componentName)),
    ])
      .then(([statusRes, componentRes]) => {
        return Promise.all([
          statusRes.json().catch(() => ({ status: 'Unknown', active_outages: [] })),
          componentRes.json().catch(() => ({ sub_components: [] })),
        ])
      })
      .then(([subStatus, componentData]) => {
        setSubComponentWithStatus({
          ...subComponent,
          status: subStatus.status,
          active_outages: subStatus.active_outages,
        })

        // Check if this subcomponent requires confirmation
        const subComponentConfig = componentData.sub_components.find(
          (sub: { name: string; requires_confirmation: boolean }) => sub.name === subComponent.name,
        )
        setRequiresConfirmation(subComponentConfig?.requires_confirmation || false)
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
        requiresConfirmation={requiresConfirmation}
      />
    </>
  )
}

export default SubComponentCardComponent
