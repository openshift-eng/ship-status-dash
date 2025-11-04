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

/**
 * Converts a slug back to a human-readable name by:
 * - Splitting on hyphens
 * - Capitalizing the first letter of each word
 * - Joining with spaces
 */
export function deslugify(slug: string): string {
  return slug
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}
