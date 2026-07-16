/** Neutral hues that avoid status greens, reds, oranges, and browns. */
const TEAM_COLORS = [
  '#5c6bc0', // indigo
  '#7e57c2', // deep purple
  '#ab47bc', // purple
  '#42a5f5', // blue
  '#29b6f6', // light blue
  '#26c6da', // cyan
  '#7986cb', // soft indigo
  '#9575cd', // soft purple
  '#64b5f6', // soft blue
  '#4fc3f7', // sky
  '#ba68c8', // orchid
  '#6a8caf', // steel blue
  '#7e8ce0', // periwinkle
  '#5c9ead', // muted blue-teal
  '#8e7cc3', // muted violet
] as const

function hashString(value: string): number {
  let hash = 0
  for (let i = 0; i < value.length; i++) {
    hash = (hash * 31 + value.charCodeAt(i)) >>> 0
  }
  return hash
}

/** Stable color for a SHIP team name. New teams get a palette color automatically. */
export function getTeamColor(team: string): string {
  return TEAM_COLORS[hashString(team) % TEAM_COLORS.length]
}
