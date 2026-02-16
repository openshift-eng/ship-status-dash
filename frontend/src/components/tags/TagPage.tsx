import { ArrowBack } from '@mui/icons-material'
import { Box, Button, CircularProgress, Container, Paper, styled, Typography } from '@mui/material'
import { useNavigate, useParams } from 'react-router-dom'

import { useTags } from '../../contexts/TagsContext'
import SubComponentList from '../sub-component/SubComponentList'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const TagHeader = styled(Paper)<{ tagColor?: string }>(({ theme, tagColor }) => ({
  padding: theme.spacing(4),
  marginBottom: theme.spacing(4),
  borderRadius: theme.spacing(2),
  backgroundColor: tagColor
    ? `${tagColor}15`
    : theme.palette.mode === 'dark'
      ? theme.palette.grey[800]
      : theme.palette.grey[100],
  border: tagColor ? `2px solid ${tagColor}40` : undefined,
}))

const TagTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '2rem',
  color: theme.palette.text.primary,
  marginBottom: theme.spacing(1),
  [theme.breakpoints.down('md')]: {
    fontSize: '1.75rem',
  },
  [theme.breakpoints.down('sm')]: {
    fontSize: '1.5rem',
  },
}))

const TagDescription = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '1rem',
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

const TagPage = () => {
  const navigate = useNavigate()
  const { tag } = useParams<{ tag: string }>()
  const { getTag, loading } = useTags()

  if (!tag) return null

  const tagInfo = getTag(tag)

  return (
    <StyledContainer maxWidth="lg">
      <BackButton variant="outlined" startIcon={<ArrowBack />} onClick={() => navigate('/')}>
        Main Dashboard
      </BackButton>

      {loading && (
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      )}

      {!loading && (
        <>
          <TagHeader elevation={2} tagColor={tagInfo?.color}>
            <TagTitle>{tagInfo?.name || tag} Sub Components</TagTitle>
            {tagInfo?.description && <TagDescription>{tagInfo.description}</TagDescription>}
          </TagHeader>

          <SubComponentList filters={{ tag }} title="" showHeader={false} showContainer={false} />
        </>
      )}
    </StyledContainer>
  )
}

export default TagPage
