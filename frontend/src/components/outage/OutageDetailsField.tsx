import { Box, Typography, styled } from '@mui/material'
import type { ReactNode } from 'react'

const FieldBox = styled(Box)(({ theme }) => ({
  padding: theme.spacing(1.5),
  borderRadius: theme.spacing(1),
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[50],
  marginBottom: theme.spacing(1),
}))

const FieldLabel = styled(Typography)(() => ({
  textTransform: 'uppercase',
  fontWeight: 600,
  letterSpacing: 0.5,
}))

const FieldValue = styled(Typography)(({ theme }) => ({
  marginTop: theme.spacing(0.5),
  fontWeight: 500,
}))

const FieldValuePrimary = styled(FieldValue)(({ theme }) => ({
  color: theme.palette.primary.main,
}))

const FieldValueMonospace = styled(FieldValue)(() => ({
  fontFamily: 'monospace',
}))

const FieldValuePreWrap = styled(FieldValue)(() => ({
  whiteSpace: 'pre-wrap',
}))

interface FieldProps {
  label: string
  value: ReactNode
  valueVariant?: 'default' | 'primary' | 'monospace' | 'pre-wrap'
}

const Field = ({ label, value, valueVariant = 'default' }: FieldProps) => {
  let ValueComponent = FieldValue
  switch (valueVariant) {
    case 'primary':
      ValueComponent = FieldValuePrimary
      break
    case 'monospace':
      ValueComponent = FieldValueMonospace
      break
    case 'pre-wrap':
      ValueComponent = FieldValuePreWrap
      break
    default:
      ValueComponent = FieldValue
  }

  return (
    <FieldBox>
      <FieldLabel variant="caption" color="text.secondary">
        {label}
      </FieldLabel>
      <ValueComponent variant="body1">{value}</ValueComponent>
    </FieldBox>
  )
}

export default Field
export { FieldBox, FieldLabel }
