import { ArrowBack } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  styled,
  Typography,
} from '@mui/material'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { SubComponent, SubComponentListParams, SubComponentListItem } from '../../types'
import { getListSubComponentsEndpoint } from '../../utils/endpoints'
import { getStatusTintStyles } from '../../utils/styles'

import SubComponentCard from './SubComponentCard'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const ListHeader = styled(Paper)(({ theme }) => ({
  ...getStatusTintStyles(theme, 'Healthy', 2),
  padding: theme.spacing(4),
  marginBottom: theme.spacing(4),
  borderRadius: theme.spacing(2),
}))

const ListTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '2rem',
  color: theme.palette.text.primary,
  [theme.breakpoints.down('md')]: {
    fontSize: '1.75rem',
  },
  [theme.breakpoints.down('sm')]: {
    fontSize: '1.5rem',
  },
}))

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
  gap: theme.spacing(3),
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

function toSubComponent(item: SubComponentListItem): SubComponent {
  const rest = Object.fromEntries(
    Object.entries(item).filter(([key]) => key !== 'component_name'),
  ) as SubComponent
  return rest
}

interface SubComponentListProps {
  filters: SubComponentListParams
  title: string
}

const SubComponentList = ({ filters, title }: SubComponentListProps) => {
  const navigate = useNavigate()
  const [items, setItems] = useState<SubComponentListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    queueMicrotask(() => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }
    })
    fetch(getListSubComponentsEndpoint(filters))
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load sub-components')
        return res.json()
      })
      .then((data) => {
        if (!cancelled) setItems(data ?? [])
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load sub-components')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [filters])

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

      {error && <Alert severity="error">{error}</Alert>}

      {!loading && !error && (
        <>
          <ListHeader elevation={2}>
            <ListTitle>{title}</ListTitle>
          </ListHeader>

          {items.length === 0 ? (
            <Typography color="text.secondary">
              No sub-components match the current filters.
            </Typography>
          ) : (
            <SubComponentsGrid>
              {items.map((item) => (
                <SubComponentCard
                  key={`${item.component_name}-${item.name}`}
                  subComponent={toSubComponent(item)}
                  componentName={item.component_name}
                />
              ))}
            </SubComponentsGrid>
          )}
        </>
      )}
    </StyledContainer>
  )
}

export default SubComponentList
