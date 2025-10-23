import { CheckCircle, Warning } from '@mui/icons-material'
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  List,
  ListItem,
  ListItemText,
  styled,
  Typography,
} from '@mui/material'
import React from 'react'
import { useNavigate } from 'react-router-dom'

import type { Outage, SubComponent } from '../../types'
import { relativeTime } from '../../utils/helpers'
import { SeverityChip, StatusChip } from '../StatusColors'

const StyledDialog = styled(Dialog)(({ theme }) => ({
  '& .MuiDialog-paper': {
    borderRadius: theme.spacing(2),
  },
}))

const HeaderBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
}))

const DescriptionTypography = styled(Typography)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const StyledListItem = styled(ListItem)(() => ({
  paddingLeft: 0,
  paddingRight: 0,
}))

const OutageHeaderBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(1),
}))

const TriageNotesTypography = styled(Typography)(({ theme }) => ({
  marginTop: theme.spacing(1),
}))

const NoOutagesBox = styled(Box)(({ theme }) => ({
  textAlign: 'center',
  paddingTop: theme.spacing(4),
  paddingBottom: theme.spacing(4),
}))

interface OutageModalProps {
  open: boolean
  onClose: () => void
  selectedSubComponent: SubComponent | null
  componentName?: string
  requiresConfirmation?: boolean
}

const OutageModal: React.FC<OutageModalProps> = ({
  open,
  onClose,
  selectedSubComponent,
  componentName,
  requiresConfirmation = false,
}) => {
  const navigate = useNavigate()

  const handleViewAllOutages = () => {
    if (componentName && selectedSubComponent?.name) {
      navigate(`/${componentName}/${selectedSubComponent.name}`)
    }
  }

  const isOutageConfirmed = (outage: Outage) => {
    return outage.confirmed_at.Valid
  }

  return (
    <StyledDialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      {selectedSubComponent && (
        <>
          <DialogTitle>
            <HeaderBox>
              <Typography variant="h6">
                {componentName} / {selectedSubComponent.name}
              </Typography>
              <StatusChip
                label={selectedSubComponent.status || 'Unknown'}
                status={selectedSubComponent.status || 'Unknown'}
                variant="filled"
              />
            </HeaderBox>
          </DialogTitle>
          <DialogContent>
            <DescriptionTypography variant="body2" color="text.secondary">
              {selectedSubComponent.description}
            </DescriptionTypography>

            {selectedSubComponent.active_outages &&
            selectedSubComponent.active_outages.length > 0 ? (
              <Box>
                <Typography variant="h6" gutterBottom>
                  Active Outages ({selectedSubComponent.active_outages.length})
                </Typography>
                <List>
                  {selectedSubComponent.active_outages.map((outage: Outage, index: number) => (
                    <React.Fragment key={outage.id}>
                      <StyledListItem alignItems="flex-start">
                        <ListItemText
                          primary={
                            <OutageHeaderBox>
                              <SeverityChip
                                label={outage.severity}
                                severity={outage.severity}
                                size="small"
                                variant="outlined"
                              />
                              {requiresConfirmation && (
                                <Chip
                                  icon={isOutageConfirmed(outage) ? <CheckCircle /> : <Warning />}
                                  label={isOutageConfirmed(outage) ? 'Confirmed' : 'Unconfirmed'}
                                  color={isOutageConfirmed(outage) ? 'success' : 'warning'}
                                  size="small"
                                  variant="outlined"
                                />
                              )}
                              <Typography variant="subtitle2">
                                {outage.description || 'No description'}
                              </Typography>
                            </OutageHeaderBox>
                          }
                          secondary={
                            <Box>
                              <Typography
                                variant="caption"
                                display="block"
                                title={new Date(outage.start_time).toLocaleString()}
                              >
                                Started: {relativeTime(new Date(outage.start_time), new Date())}
                              </Typography>
                              {outage.discovered_from && (
                                <Typography variant="caption" display="block">
                                  Discovered by: {outage.discovered_from}
                                </Typography>
                              )}
                              {outage.triage_notes && (
                                <TriageNotesTypography variant="caption" display="block">
                                  Triage Notes: {outage.triage_notes}
                                </TriageNotesTypography>
                              )}
                            </Box>
                          }
                        />
                      </StyledListItem>
                      {index < (selectedSubComponent.active_outages?.length || 0) - 1 && (
                        <Divider />
                      )}
                    </React.Fragment>
                  ))}
                </List>
              </Box>
            ) : (
              <NoOutagesBox>
                <Typography variant="h6" color="text.secondary">
                  No Active Outages
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  This sub-component is currently healthy
                </Typography>
              </NoOutagesBox>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={handleViewAllOutages} variant="outlined">
              More details
            </Button>
            <Button onClick={onClose}>Close</Button>
          </DialogActions>
        </>
      )}
    </StyledDialog>
  )
}

export default OutageModal
