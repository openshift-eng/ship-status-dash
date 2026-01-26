import '@mui/material/styles'

declare module '@mui/material/styles' {
  interface Palette {
    status: {
      healthy: {
        main: string
        light: string
        dark: string
        background: string
      }
      degraded: {
        main: string
        light: string
        dark: string
        background: string
      }
      down: {
        main: string
        light: string
        dark: string
        background: string
      }
      capacityExhausted: {
        main: string
        light: string
        dark: string
        background: string
      }
      suspected: {
        main: string
        light: string
        dark: string
        background: string
      }
      partial: {
        main: string
        light: string
        dark: string
        background: string
      }
      unknown: {
        main: string
        light: string
        dark: string
        background: string
      }
    }
  }

  interface PaletteOptions {
    status?: {
      healthy?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      degraded?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      down?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      capacityExhausted?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      suspected?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      partial?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
      unknown?: {
        main?: string
        light?: string
        dark?: string
        background?: string
      }
    }
  }
}
