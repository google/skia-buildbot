describe('The sk.human functions',
  function() {
    var clock;

    afterEach(function () { if (clock) { clock.restore(); } });

    function testPad() {
      var testCases = [
        [       0, 0, "0"],
        [       1, 1, "1"],
        [      10, 1, "10"],
        [      10, 2, "10"],
        [      10, 3, "010"],
        [31558150, 8, "31558150"],
        [31558150, 9, "031558150"],
      ];
      for (var testCase of testCases) {
        assert.equal(sk.human.pad(testCase[0], testCase[1]), testCase[2]);
      }
    }

    it('should return padded integers from pad', testPad);

    function testStrDuration() {
      var testCases = [
        [       0, "  0s"],
        [       1, "  1s"],
        [      -1, "  1s"],
        [       2, "  2s"],
        [      10, " 10s"],
        [     -30, " 30s"],
        [      59, " 59s"],
        [      60, "  1m"],
        [     -61, "  1m  1s"],
        [     123, "  2m  3s"],
        [    3599, " 59m 59s"],
        [    3600, "  1h"],
        [    3601, "  1h  1s"],
        [    3659, "  1h 59s"],
        [    3660, "  1h  1m"],
        [    3661, "  1h  1m  1s"],
        [   86399, " 23h 59m 59s"],
        [   86400, "  1d"],
        [   86401, "  1d  1s"],
        [  604799, "  6d 23h 59m 59s"],
        [  604800, "  1w"],
        [31558150, " 52w  1d  6h  9m 10s"]
      ];
      for (var testCase of testCases) {
        assert.equal(sk.human.strDuration(testCase[0]), testCase[1]);
      }
    }

    it('should return human-readable duration from strDuration',
       testStrDuration);

    function testDiffDate() {
      var now = Date.now();
      clock = sinon.useFakeTimers(now);
      var testCases = [
        [          0, "0s"],  //                 0s
        [          1, "0s"],  //                 0.001s
        [        499, "0s"],  //                 0.499s
        [        500, "1s"],  //                 0.5s
        [      -1000, "1s"],  //                 1s
        [       1000, "1s"],  //                 1s
        [       2000, "2s"],  //                 2s
        [       9800, "10s"], //                 9.8s
        [     -10000, "10s"], //                10s
        [     -30000, "30s"], //                30s
        [      59000, "59s"], //                59s
        [      59499, "59s"], //                59.499s
        [      59500, "1m"],  //                59.5s
        [      60000, "1m"],  //             1m 00s
        [     -61000, "1m"],  //             1m 01s
        [     123000, "2m"],  //             2m 03s
        [    3569000, "59m"], //            59m 29s
        [    3570000, "1h"],  //            59m 30s
        [    3600000, "1h"],  //         1h 00m 00s
        [   -3601000, "1h"],  //         1h 00m 01s
        [    3659000, "1h"],  //         1h 00m 59s
        [   -3660000, "1h"],  //         1h 01m 00s
        [    5399000, "1h"],  //         1h 29m 59s
        [    5400000, "2h"],  //         1h 30m 00s
        [  -84599000, "23h"], //        23h 29m 59s
        [  -84600000, "1d"],  //        23h 30m 00s
        [  -86399000, "1d"],  //        23h 59m 59s
        [   86400000, "1d"],  //     1d 00h 00m 00s
        [ -561599000, "6d"],  //     6d 11h 59m 59s
        [ -561600000, "1w"],  //     6d 12h 00m 00s
        [  604800000, "1w"],  //  1w 0d 00h 00m 00s
        [31558150000, "52w"]  // 52w 1d 06h 09m 10s
      ];
      for (var testCase of testCases) {
        var diffMs = testCase[0];
        var expected = testCase[1];
        var ms = now + diffMs;
        assert.equal(sk.human.diffDate(ms), expected,
                     'Input is ' + ms + ', now is ' + now);
        assert.equal(sk.human.diffDate(new Date(ms).toISOString()), expected,
                     'Input is ' + new Date(ms).toISOString() +
                         ', now is ' + new Date().toISOString());
      }
    }

    it('should return human-readable duration from diffDate',
       testDiffDate);
  }
);
