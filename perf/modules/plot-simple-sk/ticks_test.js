import { ticks } from './ticks.js'

describe('ticks', function () {
    const ts = [
        new Date(2014, 6, 1, 0, 0, 0, 0),
        new Date(2014, 7, 1, 0, 0, 0, 0),
        new Date(2014, 7, 2, 0, 0, 0, 0),
        new Date(2014, 9, 1, 0, 0, 0, 0),
    ];

    it('produces good ticks', function () {
        assert.deepEqual(JSON.stringify(ticks(ts)), []);
    });
});
