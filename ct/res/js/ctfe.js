/**
 * Utility javascript functions used across the different CT FE pages.
 *
 */

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 **/
function getFormattedTimestamp(timestamp) {
  if (timestamp == 0) {
    return "<pending>";
  }
  var pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
  return new Date(String(timestamp).replace(pattern,'$1-$2-$3T$4:$5:$6')).toLocaleString();
}
