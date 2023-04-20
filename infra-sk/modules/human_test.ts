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

import { assert } from 'chai';
import * as human from './human';

describe('The human functions', () => {
  function testPad() {
    const testCases: [number, number, string][] = [
      [0, 0, '0'],
      [1, 1, '1'],
      [10, 1, '10'],
      [10, 2, '10'],
      [10, 3, '010'],
      [31558150, 8, '31558150'],
      [31558150, 9, '031558150'],
    ];
    for (const testCase of testCases) {
      assert.equal(human.pad(testCase[0], testCase[1]), testCase[2]);
    }
  }

  it('should return padded integers from pad', testPad);

  function testStrDuration() {
    const testCases: [number, string][] = [
      [0, '  0s'],
      [1, '  1s'],
      [-1, '  1s'],
      [2, '  2s'],
      [10, ' 10s'],
      [-30, ' 30s'],
      [59, ' 59s'],
      [60, '  1m'],
      [-61, '  1m  1s'],
      [123, '  2m  3s'],
      [3599, ' 59m 59s'],
      [3600, '  1h'],
      [3601, '  1h  1s'],
      [3659, '  1h 59s'],
      [3660, '  1h  1m'],
      [3661, '  1h  1m  1s'],
      [86399, ' 23h 59m 59s'],
      [86400, '  1d'],
      [86401, '  1d  1s'],
      [604799, '  6d 23h 59m 59s'],
      [604800, '  1w'],
      [31558150, ' 52w  1d  6h  9m 10s'],
    ];
    for (const testCase of testCases) {
      assert.equal(human.strDuration(testCase[0]), testCase[1]);
    }
  }

  it('should return human-readable duration from strDuration', testStrDuration);

  function testDiffDate() {
    const now = 1584972056 * 1000; // 03/23/2020 @ 2:00pm (UTC)
    const testCases: [number, string][] = [
      [0, '0s'], //            0s
      [1, '0s'], //            0.001s
      [499, '0s'], //          0.499s
      [500, '1s'], //          0.5s
      [-1000, '1s'], //        1s
      [1000, '1s'], //         1s
      [2000, '2s'], //         2s
      [9800, '10s'], //        9.8s
      [-10000, '10s'], //      10s
      [-30000, '30s'], //      30s
      [59000, '59s'], //       59s
      [59499, '59s'], //       59.499s
      [59500, '1m'], //        59.5s
      [60000, '1m'], //        1m 00s
      [-61000, '1m'], //       1m 01s
      [123000, '2m'], //       2m 03s
      [3569000, '59m'], //     59m 29s
      [3570000, '1h'], //      59m 30s
      [3600000, '1h'], //      1h 00m 00s
      [-3601000, '1h'], //     1h 00m 01s
      [3659000, '1h'], //      1h 00m 59s
      [-3660000, '1h'], //     1h 01m 00s
      [5398000, '1h'], //      1h 29m 58s
      [5400000, '2h'], //      1h 30m 00s
      [-84599000, '23h'], //   23h 29m 59s
      [-84600000, '1d'], //    23h 30m 00s
      [-86399000, '1d'], //    23h 59m 59s
      [86400000, '1d'], //     1d 00h 00m 00s
      [-561599000, '6d'], //   6d 11h 59m 59s
      [-561600000, '1w'], //   6d 12h 00m 00s
      [604800000, '1w'], //    1w 0d 00h 00m 00s
      [31558150000, '52w'], // 52w 1d 06h 09m 10s
    ];
    for (const testCase of testCases) {
      const diffMs = testCase[0];
      const expected = testCase[1];
      const ms = now + diffMs;
      // Test the form of diffDate that takes a number.
      assert.equal(
        human.diffDate(ms, now),
        expected,
        `Input is ${ms}, now is ${now}`
      );
      // Test the form of diffDate that takes a date string.
      assert.equal(
        human.diffDate(new Date(ms).toISOString(), now),
        expected,
        `Input is ${new Date(ms).toISOString()}, now is ${new Date(
          now
        ).toISOString()}`
      );
    }
  }

  it('should return human-readable duration from diffDate', testDiffDate);

  function testBytes() {
    const testBytesTestCases: [number, string][] = [
      [0, '0 B'], //              0 B
      [1, '1 B'], //              1 B
      [499, '499 B'], //          499 B
      [500, '500 B'], //          500 B
      [1000, '1000 B'], //        1000 B
      [1234, '1 KB'], //          1 KB 210 B
      [2000, '2 KB'], //          1 KB 976 B
      [9727, '9 KB'], //          9 KB 511 B
      [9728, '10 KB'], //         9 KB 512 B
      [30000, '29 KB'], //        29 KB 304 B
      [1024000, '1000 KB'], //    1000 KB 000 B
      [1048500, '1 MB'], //       1023 KB 948 B
      [1048576, '1 MB'], //       1 MB 000 KB 000 B
      [1048577, '1 MB'], //       1 MB 000 KB 001 B
      [300000000, '286 MB'], //   286 MB 104 KB 768 B
      [1072693248, '1023 MB'], // 1023 MB 000 KB 000 B
      [1073741300, '1 GB'], //    1023 MB1023 KB 999 B
      [1073741824, '1 GB'], //    1 GB 000 MB 000 KB 000 B
      [1073741825, '1 GB'], //    1 GB 000 MB 000 KB 001 B
    ];
    for (const tb of testBytesTestCases) {
      const b = tb[0];
      const expected = tb[1];
      assert.equal(human.bytes(b), expected, `Input is ${b}, Unit is bytes`);
    }
    const testMB: [number, string][] = [
      [0, '0 B'], //              0 MB
      [1, '1 MB'], //             1 MB
      [499, '499 MB'], //         499 MB
      [500, '500 MB'], //         500 MB
      [1000, '1000 MB'], //       1000 MB
      [1234, '1 GB'], //          1 GB 210 MB
      [2000, '2 GB'], //          1 GB 976 MB
      [9727, '9 GB'], //          9 GB 511 MB
      [9728, '10 GB'], //         9 GB 512 MB
      [30000, '29 GB'], //        29 GB 304 MB
      [1024000, '1000 GB'], //    1000 GB 000 MB
      [1048500, '1 TB'], //       1023 GB 948 MB
      [1048576, '1 TB'], //       1 TB 000 GB 000 MB
      [1048577, '1 TB'], //       1 TB 000 GB 001 MB
      [300000000, '286 TB'], //   286 TB 104 GB 768 MB
      [1072693248, '1023 TB'], // 1023 TB 000 GB 000 MB
      [1073741300, '1 PB'], //    1023 TB1023 GB 999 MB
      [1073741824, '1 PB'], //    1 PB 000 TB 000 GB 000 MB
      [1073741825, '1 PB'], //    1 PB 000 TB 000 GB 001 MB
    ];
    for (const tm of testMB) {
      const b = tm[0];
      const expected = tm[1];
      assert.equal(
        human.bytes(b, human.MB),
        expected,
        `Input is ${b}, Unit is Megabytes`
      );
    }
  }

  it('should return human-readable bytes from bytes', testBytes);
});
