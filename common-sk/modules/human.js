/** @module common-sk/modules/human
 *  @description Utilities for working with human friendly I/O.
 */

const TIME_DELTAS = [
  { units: "w", delta: 7*24*60*60 },
  { units: "d", delta:   24*60*60 },
  { units: "h", delta:      60*60 },
  { units: "m", delta:         60 },
  { units: "s", delta:          1 },
];

/** @constant {number} */
export const KB = 1024;
/** @constant {number} */
export const MB = KB * 1024;
/** @constant {number} */
export const GB = MB * 1024;
/** @constant {number} */
export const TB = GB * 1024;
/** @constant {number} */
export const PB = TB * 1024;

const BYTES_DELTAS = [
  { units: " PB", delta: PB},
  { units: " TB", delta: TB},
  { units: " GB", delta: GB},
  { units: " MB", delta: MB},
  { units: " KB", delta: KB},
  { units: " B",  delta:  1},
];

/** Left pad a number with 0's.
 *
 * @param {number} num - The number to pad.
 * @param {number} size - The number of digits to pad out to.
 * @returns {string}
 */
export function pad(num, size) {
  let str = num + "";
  while (str.length < size) str = "0" + str;
  return str;
}

/**
 * Returns a human-readable format of the given duration in seconds.
 * For example, 'strDuration(123)' would return "2m 3s".
 * Negative seconds is treated the same as positive seconds.
 *
 * @param {number} seconds - The duration.
 * @returns {string}
 */
export function strDuration(seconds) {
  if (seconds < 0) {
    seconds = -seconds;
  }
  if (seconds === 0) { return '  0s'; }
  let rv = "";
  for (let i=0; i<TIME_DELTAS.length; i++) {
    if (TIME_DELTAS[i].delta <= seconds) {
      let s = Math.floor(seconds/TIME_DELTAS[i].delta)+TIME_DELTAS[i].units;
      while (s.length < 4) {
        s = ' ' + s;
      }
      rv += s;
      seconds = seconds % TIME_DELTAS[i].delta;
    }
  }
  return rv;
}

/**
 * Returns the difference between the current time and 's' as a string in a
 * human friendly format. If 's' is a number it is assumed to contain the time
 * in milliseconds otherwise it is assumed to contain a time string parsable
 * by Date.parse().
 *
 * For example, a difference of 123 seconds between 's' and the current time
 * would return "2m".
 *
 * @param {Object} milliseconds - The time in milliseconds or a time string.
 * @returns {string}
 */
export function diffDate(s) {
  let ms = (typeof(s) === "number") ? s : Date.parse(s);
  let diff = (ms - Date.now())/1000;
  if (diff < 0) {
    diff = -1.0 * diff;
  }
  return humanize(diff, TIME_DELTAS);
}

/**
 * Formats the amount of bytes in a human friendly format.
 * unit may be supplied to indicate b is not in bytes, but in something
 * like kilobytes (KB) or megabytes (MB)
 *
 * @example
 * // returns "1 KB"
 * bytes(1234)
 * @example
 * // returns "5 GB"
 * bytes(5321, MB)
 *
 * @param {number} b - The number of bytes in units 'unit'.
 * @param {number} unit - The number of bytes per unit.
 * @returns {string}
 */
export function bytes(b, unit = 1) {
  if (Number.isInteger(unit)) {
    b = b * unit;
  }
  return humanize(b, BYTES_DELTAS);
}

/** localeTime formats the provided Date object in locale time and appends the timezone to the end.
 *
 * @param {Date} date
 * @returns {string}
 */
export function localeTime(date) {
  // caching timezone could be buggy, especially if times from a wide range
  // of dates are used. The main concern would be crossing over Daylight
  // Savings time and having some times be erroneously in EST instead of
  // EDT, for example
  let str = date.toString();
  let timezone = str.substring(str.indexOf("("));
  return date.toLocaleString() + " " + timezone;
}


function humanize(n, deltas) {
  for (let i=0; i<deltas.length-1; i++) {
    // If n would round to '60s', return '1m' instead.
    let nextDeltaRounded =
      Math.round(n/deltas[i+1].delta)*deltas[i+1].delta;
    if (nextDeltaRounded/deltas[i].delta >= 1) {
      return Math.round(n/deltas[i].delta)+deltas[i].units;
    }
  }
  let i = deltas.length-1;
  return Math.round(n/deltas[i].delta)+deltas[i].units;
}
