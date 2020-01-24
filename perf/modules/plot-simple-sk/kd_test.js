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

import { kdTree } from './kd.js'

describe('kdTree search', function () {

    const points = [
        { x: 1, y: 2 },
        { x: 3, y: 4 },
        { x: 5, y: 6 },
        { x: 7, y: 8 }
    ];

    const distance = function (a, b) {
        return Math.pow(a.x - b.x, 2) + Math.pow(a.y - b.y, 2);
    }

    const tree = new kdTree(points, distance, ["x", "y"]);

    it('finds the closest point', function () {
        let nearest = tree.nearest({ x: 5, y: 5 }, 1)[0][0];
        assert.deepEqual(nearest, { x: 5, y: 6 });
        nearest = tree.nearest({ x: 1, y: 1 }, 1)[0][0];
        assert.deepEqual(nearest, { x: 1, y: 2 });

    });
});
