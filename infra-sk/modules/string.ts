/**
 * @module infra-sk/modules/string
 * @description Shared utilities for strings.
 */

const ellipsis = '...';

/**
 * Truncate the given string to the given length. If the string was
 * shortened, change the last three characters to ellipsis.
 */
export function truncate(str: string, len: number) {
  if (str.length <= len) {
    return str;
  }
  if (len <= ellipsis.length) {
    return str.substring(0, len);
  }
  return str.substring(0, len - ellipsis.length) + ellipsis;
}
