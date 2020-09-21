/**
 * @module ticks
 * @description Function for creating tick marks for a range of times.
 */

/**
 * @constant {Number} Minimum number of ticks to return. We don't guarantee we
 * will meet this.
 */
const MIN_TICKS = 2;

/**
 * @constant {Number} Maximum number of ticks to return. We will always meet
 * this limit.
 */
const MAX_TICKS = 10;

/**
 * @constant {Array} - choices is the list of duration increments and associated
 * formatters for those times, sorted from largest to smallest duration.
 */
const choices = [
  {
    duration: 4 * 7 * 24 * 60 * 60 * 1000,
    formatter: new Intl.DateTimeFormat('default', { month: 'short' }).format,
  },
  {
    duration: 3 * 24 * 60 * 60 * 1000,
    formatter: new Intl.DateTimeFormat('default', {
      day: 'numeric',
      month: 'short',
    }).format,
  },
  {
    duration: 24 * 60 * 60 * 1000,
    formatter: new Intl.DateTimeFormat('default', {
      weekday: 'short',
      hour: 'numeric',
    }).format,
  },
  {
    duration: 2 * 60 * 60 * 1000,
    formatter: new Intl.DateTimeFormat('default', { hour: 'numeric' }).format,
  },
  {
    duration: 2 * 60 * 1000,
    formatter: new Intl.DateTimeFormat('default', {
      hour: 'numeric',
      minute: 'numeric',
    }).format,
  },
  {
    duration: 2 * 1000,
    formatter: new Intl.DateTimeFormat('default', {
      hour: 'numeric',
      minute: 'numeric',
      second: 'numeric',
    }).format,
  },
];

/**
 *  formatterFromDuration takes a duration in milliseconds and from that returns
 *  a function that will produce good labels for tick marks in that time range.
 *  For example, if the time range is small enough then the ticks will be marked
 *  with the weekday, e.g.  "Sun", if the time range is much larger the ticks
 *  may be marked with the month, e.g. "Jul".
 *
 *  @param {Number} ms - A duration in milliseconds.
 */
function formatterFromDuration(ms: number) {
  // Move down the list of choices from the largest granularity to the finest.
  // The first one that would generate more than MIN_TICKS for the given
  // number of hours is chosen and that TimeOp is returned.
  for (let i = 0; i < choices.length; i++) {
    const c = choices[i];
    if (ms / c.duration > MIN_TICKS) {
      return c.formatter;
    }
  }
  return choices[choices.length - 1].formatter;
}

export interface tick {
  x: number;
  text: string;
}

/**
 * ticks takes a set of times that represent x-axis locations in time
 * and returns an array of points to use to for tick marks along with their
 * associated text.
 *
 * @param {Date[]} dates - An array of Dates, one for each x location.
 * @returns {Array} An array of objects of the form:
 *
 *     {
 *       x: 2,
 *       text: 'Mon, 8 AM',
 *     }
 */
export function ticks(dates: Date[]): tick[] {
  if (dates.length === 0) {
    return [];
  }
  const duration = dates[dates.length - 1].valueOf() - dates[0].valueOf();
  const formatter = formatterFromDuration(duration);
  let last = formatter(dates[0]);
  let ret = [
    {
      x: 0,
      text: last,
    },
  ];
  for (let i = 0; i < dates.length; i++) {
    const tickValue = formatter(dates[i]);
    if (last !== tickValue) {
      ret.push({
        x: i,
        text: tickValue,
      });
      last = tickValue;
    }
  }
  // Drop every other tick repeatedly until we get less than MAX_TICKS tick marks.
  while (ret.length > MAX_TICKS) {
    ret = ret.filter((t, i) => i % 2);
  }
  return ret;
}
