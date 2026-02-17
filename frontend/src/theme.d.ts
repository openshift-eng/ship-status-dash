import '@mui/material/styles'

type StatusColor = {
  main: string
  light: string
  dark: string
  background: string
}

declare module '@mui/material/styles' {
  interface Palette {
    status: {
      healthy: StatusColor
      degraded: StatusColor
      down: StatusColor
      capacityExhausted: StatusColor
      suspected: StatusColor
      partial: StatusColor
      unknown: StatusColor
    }
    tagBorderColor: string
    tagBackgroundColor: string
    tagTextColor: string
  }

  interface PaletteOptions {
    status?: {
      healthy?: StatusColor
      degraded?: StatusColor
      down?: StatusColor
      capacityExhausted?: StatusColor
      suspected?: StatusColor
      partial?: StatusColor
      unknown?: StatusColor
    }
    tagBorderColor?: string
    tagBackgroundColor?: string
    tagTextColor?: string
  }
}
