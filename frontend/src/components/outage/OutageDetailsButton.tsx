import { Visibility } from '@mui/icons-material'
import { Button, Tooltip } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import type { Outage } from '../../types'

interface OutageDetailsButtonProps {
  outage: Outage
}

const OutageDetailsButton = ({ outage }: OutageDetailsButtonProps) => {
  const navigate = useNavigate()

  const handleDetailsClick = () => {
    navigate(`/${outage.component_name}/${outage.sub_component_name}/outages/${outage.id}`)
  }

  return (
    <Tooltip title="View full details" arrow>
      <Button size="small" onClick={handleDetailsClick} startIcon={<Visibility />}></Button>
    </Tooltip>
  )
}

export default OutageDetailsButton
