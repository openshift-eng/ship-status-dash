import { Chip, styled } from '@mui/material'
import { lighten } from '@mui/material/styles'
import { Link } from 'react-router-dom'

const StyledTagChip = styled(Chip)<{ size?: 'small' | 'medium'; tagColor?: string }>(({
  theme,
  size,
  tagColor,
}) => {
  const isDark = theme.palette.mode === 'dark'
  const textColor = tagColor
    ? isDark
      ? lighten(tagColor, 0.4)
      : tagColor
    : theme.palette.tagTextColor

  return {
    backgroundColor: tagColor
      ? `${tagColor}${isDark ? '25' : '15'}`
      : theme.palette.tagBackgroundColor,
    color: textColor,
    border: tagColor ? `1px solid ${tagColor}40` : `1px solid ${theme.palette.tagBorderColor}`,
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
  }
})

interface TagChipProps {
  tag: string
  size?: 'small' | 'medium'
  color?: string
}

const TagChip = ({ tag, size = 'medium', color }: TagChipProps) => (
  <Link
    to={`/tags/${encodeURIComponent(tag)}`}
    style={{ textDecoration: 'none' }}
    onClick={(e) => e.stopPropagation()}
  >
    <StyledTagChip label={tag} size={size} tagColor={color} />
  </Link>
)

export default TagChip
