/**
 * @module infra-sk/modules/string
 * @description Shared utilities for strings.
 */

/**
 * Truncate the given string to the given length. If the string was
 * shortened, change the last three characters to ellipsis.
 */
export function truncate(str: string, len: number) {
  if (str.length > len) {
    const ellipsis = "..."
    return str.substring(0, len - ellipsis.length) + ellipsis;
  }
  return str
}