import { ticks } from './ticks.js';

let ts = [
    new Date(2014, 6, 1, 0, 0, 0, 0),
    new Date(2014, 7, 1, 0, 0, 0, 0),
    new Date(2014, 7, 2, 0, 0, 0, 0),
    new Date(2014, 9, 1, 0, 0, 0, 0),
];

console.log(JSON.stringify(ticks(ts)));

ts = [
    new Date(2014, 6, 1, 10, 0, 0, 0),
    new Date(2014, 6, 1, 12, 0, 0, 0),
    new Date(2014, 6, 1, 14, 0, 0, 0),
    new Date(2014, 6, 1, 14, 10, 0, 0),
];

console.log(JSON.stringify(ticks(ts)));

ts = [
    new Date(2014, 6, 1, 10, 5, 0, 0),
    new Date(2014, 6, 1, 10, 6, 0, 0),
    new Date(2014, 6, 1, 10, 9, 0, 0),
    new Date(2014, 6, 1, 10, 30, 0, 0),
];

console.log(JSON.stringify(ticks(ts)));


ts = [
    new Date(2014, 6, 1, 10, 5, 0, 0),
    new Date(2014, 6, 3, 10, 6, 0, 0),
    new Date(2014, 6, 5, 10, 9, 0, 0),
    new Date(2014, 6, 7, 10, 30, 0, 0),
];

console.log(JSON.stringify(ticks(ts)));

ts = [
    new Date(2014, 6, 1, 10, 5, 5, 0),
    new Date(2014, 6, 1, 10, 5, 6, 0),
    new Date(2014, 6, 1, 10, 5, 10, 0),
    new Date(2014, 6, 1, 10, 5, 20, 0),
];

console.log(JSON.stringify(ticks(ts)));

ts = [
    new Date(2014, 6, 1, 0, 0, 0, 0),
    new Date(2014, 6, 2, 0, 0, 0, 0),
    new Date(2014, 6, 2, 10, 0, 0, 0),
    new Date(2014, 6, 3, 0, 0, 0, 0),
];

console.log(JSON.stringify(ticks(ts)));