/**
 * @module ticks
 * @description Functions for creating tick marks for a range of times.
 */

const MIN_TICKS = 2;

const month = new Intl.DateTimeFormat('default', { month: 'short' }).format;
const dayOfMonth = new Intl.DateTimeFormat('default', { day: 'numeric', month: 'short' }).format;
const weekday = new Intl.DateTimeFormat('default', { weekday: 'short', hour: 'numeric' }).format;
const hour = new Intl.DateTimeFormat('default', { weekday: 'short', hour: 'numeric' }).format;
const minute = new Intl.DateTimeFormat('default', { hour: 'numeric', minute: 'numeric' }).format;
const second = new Intl.DateTimeFormat('default', { hour: 'numeric', minute: 'numeric', second: 'numeric' }).format;

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

// formatterFromDuration takes a number of hours and from that returns a
// function that will produce good labels for tick marks in that time range. For
// example, if the time range is small enough then the ticks will be marked with
// the weekday, e.g.  "Sun", if the time range is much larger the ticks may be
// marked with the month, e.g. "Jul".
function formatterFromDuration(ms) {
    // Move down the list of choices from the largest granularity to the finest.
    // The first one that would generate more than MIN_TICKS for the given number
    // of hours is chosen and that TimeOp is returned.
    for (let i = 0; i < choices.length; i++) {
        let c = choices[i]
        if (ms / c.duration > MIN_TICKS) {
            return c.formatter;
        }
    }
    return choices[choices.length - 1].formatter;
}

export function ticks(dates) {
    const duration = dates[dates.length - 1] - dates[0];
    const formatter = formatterFromDuration(duration);
    let last = formatter(dates[0]);
    const ret = [{
        x: 0,
        text: last,
    }];
    for (let i = 0; i < dates.length; i++) {
        const tickValue = formatter(dates[i]);
        if (last != tickValue) {
            ret.push({
                x: i,
                text: tickValue,
            })
            last = tickValue;
        }
    }
    return ret;
}
