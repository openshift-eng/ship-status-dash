/**
 * Converts a string to a URL-safe slug by:
 * - Converting to lowercase
 * - Replacing spaces and special characters with hyphens
 * - Removing multiple consecutive hyphens
 * - Trimming hyphens from start and end
 */
export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}
