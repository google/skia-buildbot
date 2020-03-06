/**
 * Utility javascript functions used across the different CT FE pages.
 */
import { pad } from 'common-sk/modules/human'

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 **/
export function getFormattedTimestamp(timestamp) {
  if (timestamp == 0) {
    return "<pending>";
  }
  return getTimestamp(timestamp).toLocaleString();
}

/**
 * Converts the timestamp used in CTFE DB into a Javascript timestamp.
 */
export function getTimestamp(timestamp) {
  if (timestamp == 0) {
    return timestamp;
  }
  var pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
  return new Date(String(timestamp).replace(pattern,'$1-$2-$3T$4:$5:$6'));
}

/**
 * Convert from Javascript Date to timestamp recognized by CTFE DB.
 */
export function getCtDbTimestamp(d) {
  var timestamp = String(d.getUTCFullYear()) + pad(d.getUTCMonth()+1, 2) +
                  pad(d.getUTCDate(), 2) + pad(d.getUTCHours(), 2) +
                  pad(d.getUTCMinutes(), 2) + pad(d.getUTCSeconds(), 2);
  return timestamp
}
