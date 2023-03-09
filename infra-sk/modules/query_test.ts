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
import * as query from './query';

describe('Test query encoding and decoding functions.', () => {
  function testEncode() {
    assert.equal(query.fromObject({}), '');
    assert.equal(query.fromObject({ a: 2 }), 'a=2');
    assert.equal(query.fromObject({ a: '2' }), 'a=2');
    assert.equal(query.fromObject({ a: '2 3' }), 'a=2%203');
    assert.equal(query.fromObject({ 'a b': '2 3' }), 'a%20b=2%203');
    assert.equal(query.fromObject({ a: [2, 3] }), 'a=2&a=3');
    assert.equal(query.fromObject({ a: ['2', '3'] }), 'a=2&a=3');
    assert.equal(query.fromObject({ a: [] }), '');
    assert.equal(query.fromObject({ a: { b: '3' } }), 'a=b%3D3');
    assert.equal(query.fromObject({ a: { b: '3' }, b: '3' }), 'a=b%3D3&b=3');
    assert.equal(query.fromObject({ a: {}, b: '3' }), 'a=&b=3');
    assert.equal(
      query.fromObject({ a: { b: { c: 'foo bar' } } }),
      'a=b%3Dc%253Dfoo%252520bar',
    );
    assert.isTrue(
      ['a=2&b=3', 'b=3&a=2'].indexOf(query.fromObject({ a: 2, b: 3 })) !== -1,
    );
  }

  function testDecodeToObject() {
    assert.deepEqual(query.toObject('', {}), {});
    assert.deepEqual(query.toObject('a=2', {}), { a: '2' });
    assert.deepEqual(query.toObject('a=2', { a: 'foo' }), { a: '2' });
    assert.deepEqual(query.toObject('a=2', { a: 1.0 }), { a: 2 });
    assert.deepEqual(query.toObject('a=true', { a: false }), { a: true });
    assert.deepEqual(query.toObject('a=true', { a: 'bar' }), { a: 'true' });
    assert.deepEqual(query.toObject('a=false', { a: false }), { a: false });
    assert.deepEqual(query.toObject('a=baz', { a: 2.0 }), { a: NaN });
    assert.deepEqual(query.toObject('a=true&a=false', { a: [] }), {
      a: ['true', 'false'],
    });
    assert.deepEqual(query.toObject('a=true%20false', { a: [] }), {
      a: ['true false'],
    });
    assert.deepEqual(
      query.toObject('b=1&a=true%20false&b=2.2', { a: [], b: [] }),
      { a: ['true false'], b: ['1', '2.2'] },
    );
    assert.deepEqual(
      query.toObject('a=b%3Dc%253Dfoo%252520bar', { a: { b: { c: '' } } }),
      { a: { b: { c: 'foo bar' } } },
    );

    assert.deepEqual(query.toObject('a=2&b=true', { a: 1.0, b: false }), {
      a: 2,
      b: true,
    });
  }

  function testRoundTrip() {
    const start: any = {
      a: 2.0,
      b: true,
      c: 'foo bar baz',
      e: ['foo bar', '2'],
      d: ['foo'],
      f: { a: 2.0, b: 'foo bar', c: ['a', 'b'] },
    };
    const hint: any = {
      a: 0,
      b: false,
      c: 'string',
      d: [],
      e: [],
      f: { a: 1.0, b: 'string', c: [] },
    };
    assert.deepEqual(query.toObject(query.fromObject(start), hint), start);
  }

  function testDecodeToParamSet() {
    assert.deepEqual(query.toParamSet(''), {});
    assert.deepEqual(query.toParamSet('a=2'), { a: ['2'] });
    assert.deepEqual(query.toParamSet('a=2&a=3'), { a: ['2', '3'] });
    assert.deepEqual(query.toParamSet('a=2&a=3&b=foo'), {
      a: ['2', '3'],
      b: ['foo'],
    });
    assert.deepEqual(query.toParamSet('a=2%20'), { a: ['2 '] });
  }

  function testEncodeFromParamSet() {
    assert.deepEqual(query.fromParamSet({}), '');
    assert.deepEqual(query.fromParamSet({ a: ['2'] }), 'a=2');
    assert.deepEqual(query.fromParamSet({ a: ['2', '3'] }), 'a=2&a=3');
    assert.deepEqual(
      query.fromParamSet({ a: ['2', '3'], b: ['foo'] }),
      'a=2&a=3&b=foo',
    );
    assert.deepEqual(query.fromParamSet({ a: ['2 '] }), 'a=2%20');
  }

  it('should be able to encode and decode objects.', () => {
    testEncode();
    testDecodeToObject();
    testRoundTrip();
    testDecodeToParamSet();
    testEncodeFromParamSet();
  });
});
