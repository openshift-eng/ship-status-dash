import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import type { ReactNode } from 'react'

const SectionCard = styled(Card)(({ theme }) => ({
  height: '100%',
  borderRadius: theme.spacing(1.5),
  boxShadow: theme.shadows[2],
  transition: 'all 0.2s ease-in-out',
  '&:hover': {
    boxShadow: theme.shadows[4],
    transform: 'translateY(-2px)',
  },
}))

const SectionHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(2),
  paddingBottom: theme.spacing(1.5),
  borderBottom: `2px solid ${theme.palette.divider}`,
}))

const SectionTitle = styled(Typography)(() => ({
  fontWeight: 600,
}))

const SectionIconBox = styled(Box)(() => ({
  fontSize: 20,
  display: 'flex',
  alignItems: 'center',
}))

interface SectionProps {
  icon: ReactNode
  title: string
  children: ReactNode
}

const Section = ({ icon, title, children }: SectionProps) => (
  <SectionCard>
    <CardContent>
      <SectionHeader>
        <SectionIconBox>{icon}</SectionIconBox>
        <SectionTitle variant="h6">{title}</SectionTitle>
      </SectionHeader>
      <Box display="flex" flexDirection="column" gap={1.5}>
        {children}
      </Box>
    </CardContent>
  </SectionCard>
)

export default Section
