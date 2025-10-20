import React from 'react'
import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import { Component, SubComponent } from '../types'
import SubComponentCard from './SubComponentCard'
import { StatusChip } from './StatusColors'
import { getStatusBackgroundColor } from '../utils/helpers'

const ComponentWell = styled(Card)<{ status: string }>(({ theme, status }) => {
  const color = getStatusBackgroundColor(theme, status)

  return {
    backgroundColor: color,
    border: `2px solid ${color}`,
    borderRadius: theme.spacing(2),
    transition: 'all 0.2s ease-in-out',
    '&:hover': {
      boxShadow: theme.shadows[6],
      transform: 'translateY(-2px)',
    },
  }
})

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
  gap: theme.spacing(2),
  marginTop: theme.spacing(2),
}))

const HeaderBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: theme.spacing(3),
  paddingBottom: theme.spacing(2),
  borderBottom: `1px solid ${theme.palette.divider}`,
}))

const ComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1.5rem',
  color: theme.palette.text.primary,
}))

const DescriptionTypography = styled(Typography)(({ theme }) => ({
  marginBottom: theme.spacing(3),
  fontSize: '1rem',
  lineHeight: 1.6,
  color: theme.palette.text.secondary,
}))

interface ComponentWellProps {
  component: Component
}

const ComponentWellComponent: React.FC<ComponentWellProps> = ({ component }) => {
  return (
    <ComponentWell status={component.status || 'Unknown'}>
      <CardContent>
        <HeaderBox>
          <ComponentTitle>
            {component.name}
          </ComponentTitle>
          <StatusChip
            label={component.status || 'Unknown'}
            status={component.status || 'Unknown'}
            variant="filled"
          />
        </HeaderBox>

        <DescriptionTypography>
          {component.description}
        </DescriptionTypography>

        <SubComponentsGrid>
          {component.sub_components.map((subComponent: SubComponent) => (
            <SubComponentCard
              key={subComponent.name}
              subComponent={subComponent}
              componentName={component.name}
            />
          ))}
        </SubComponentsGrid>
      </CardContent>
    </ComponentWell>
  )
}

export default ComponentWellComponent
