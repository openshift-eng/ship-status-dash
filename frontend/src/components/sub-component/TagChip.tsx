import { Chip, styled } from '@mui/material'

const StyledTagChip = styled(Chip)<{ size?: 'small' | 'medium' }>(({ theme, size }) => ({
  backgroundColor: theme.palette.tagBackgroundColor,
  color: theme.palette.tagTextColor,
  border: `1px solid ${theme.palette.tagBorderColor}`,
  transition: 'transform 0.2s ease, box-shadow 0.2s ease',
  ...(size === 'small' && {
    fontSize: '0.65rem',
    height: '20px',
    '& .MuiChip-label': {
      padding: '0 8px',
    },
  }),
  '&:hover': {
    transform: 'scale(1.1)',
    boxShadow: theme.shadows[4],
  },
}))

interface TagChipProps {
  tag: string
  size?: 'small' | 'medium'
}

const TagChip = ({ tag, size = 'medium' }: TagChipProps) => {
  return <StyledTagChip label={tag} size={size} />
}

export default TagChip
