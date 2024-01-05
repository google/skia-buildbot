// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/** @module common-sk/modules/human
 *  @description Utitities for working with human friendly I/O.
 */

interface Delta {
  readonly units: string;
  readonly delta: number;
}

const TIME_DELTAS: Delta[] = [
  { units: 'w', delta: 7 * 24 * 60 * 60 },
  { units: 'd', delta: 24 * 60 * 60 },
  { units: 'h', delta: 60 * 60 },
  { units: 'm', delta: 60 },
  { units: 's', delta: 1 },
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

const BYTES_DELTAS: Delta[] = [
  { units: ' PB', delta: PB },
  { units: ' TB', delta: TB },
  { units: ' GB', delta: GB },
  { units: ' MB', delta: MB },
  { units: ' KB', delta: KB },
  { units: ' B', delta: 1 },
];

/** Left pad a number with 0s.
 *
 * @param num - The number to pad.
 * @param size - The number of digits to pad out to.
 */
export function pad(num: number, size: number): string {
  let str = `${num}`;
  while (str.length < size) {
    str = `0${str}`;
  }
  return str;
}

/**
 * Returns a human-readable format of the given duration in seconds.
 * For example, 'strDuration(123)' would return "2m 3s".
 * Negative seconds is treated the same as positive seconds.
 *
 * @param seconds - The duration.
 */
export function strDuration(seconds: number): string {
  if (seconds < 0) {
    seconds = -seconds;
  }
  if (seconds === 0) {
    return '  0s';
  }
  let rv = '';
  for (const td of TIME_DELTAS) {
    if (td.delta <= seconds) {
      let s = Math.floor(seconds / td.delta) + td.units;
      while (s.length < 4) {
        s = ` ${s}`;
      }
      rv += s;
      seconds %= td.delta;
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
 * @param milliseconds - The time in milliseconds or a time string.
 * @param now - The time to diff against, if not supplied then the diff
 * is done against Date.now().
 */
export function diffDate(s: number | string, now?: number): string {
  if (now === undefined) {
    now = Date.now();
  }
  const ms = typeof s === 'number' ? s : Date.parse(s);
  let diff = (ms - now) / 1000;
  if (diff < 0) {
    // eslint-disable-next-line operator-assignment
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
 * @param b - The number of bytes in units 'unit'.
 * @param unit - The number of bytes per unit.
 */
export function bytes(b: number, unit: number = 1): string {
  return humanize(b * unit, BYTES_DELTAS);
}

/** localeTime formats the provided Date object in locale time and appends the timezone to the end.
 *
 * @param date The date to format.
 */
export function localeTime(date: Date): string {
  // caching timezone could be buggy, especially if times from a wide range
  // of dates are used. The main concern would be crossing over Daylight
  // Savings time and having some times be erroneously in EST instead of
  // EDT, for example
  const str = date.toString();
  const timezone = str.substring(str.indexOf('('));
  return `${date.toLocaleString()} ${timezone}`;
}

function humanize(n: number, deltas: Delta[]) {
  for (let i = 0; i < deltas.length - 1; i++) {
    // If n would round to '60s', return '1m' instead.
    const nextDeltaRounded =
      Math.round(n / deltas[i + 1].delta) * deltas[i + 1].delta;
    if (nextDeltaRounded / deltas[i].delta >= 1) {
      return Math.round(n / deltas[i].delta) + deltas[i].units;
    }
  }
  const index = deltas.length - 1;
  return Math.round(n / deltas[index].delta) + deltas[index].units;
}
