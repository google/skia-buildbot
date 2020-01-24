/**
 * @module ticks
 * @description Functions for creating tick marks for a range of times.
 */

const MIN_TICKS = 2;

const month = new Intl.DateTimeFormat('default', { month: 'short' });
const dayOfMonth = new Intl.DateTimeFormat('default', { day: 'numeric', month: 'short' });
const weekday = new Intl.DateTimeFormat('default', { weekday: 'short' });
const hour = new Intl.DateTimeFormat('default', { hour: 'numeric' });
const minute = new Intl.DateTimeFormat('default', { hour: 'numeric', minute: 'numeric' });
const second = new Intl.DateTimeFormat('default', { hour: 'numeric', minute: 'numeric', second: 'numeric' });

// choices is the list of duration increments and associated formatters for
// those times, sorted from largest to smallest duration.
const choices = [
    {
        duration: 4 * 7 * 24 * 60 * 60 * 1000,
        formatter: month,
    },
    {
        duration: 3 * 24 * 60 * 60 * 1000,
        formatter: dayOfMonth,
    },
    {
        duration: 24 * 60 * 60 * 1000,
        formatter: weekday,
    },
    {
        duration: 2 * 60 * 60 * 1000,
        formatter: hour
    },
    {
        duration: 2 * 60 * 1000,
        formatter: minute,
    },
    {
        duration: 2 * 1000,
        formatter: second,
    },
]

// formatterFromDuration takes a number of hours and from that returns a function that
// will produce good labels for tick marks in that time range. For example, if
// the time range is small enough then the ticks will be marked with the
// weekday, e.g.  "Sun", if the time range is much larger the ticks may be
// marked with the month, e.g. "Jul".
function formatterFromDuration(ms) {
    // Move down the list of choices from the largest granularity to the finest.
    // The first one that would generate more than MIN_TICKS for the given number
    // of hours is chosen and that TimeOp is returned.
    for (let i = 0; i < choices.length; i++) {
        let c = choices[i]
        if (ms > c.duration) {
            return c.formatter;
        }
    }
    return choices[choices.length - 1].formatter
}


