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

import './index';
import { assert } from 'chai';
import { CheckOrRadio } from './checkbox-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('checkbox-sk', () => {
  const newInstance = setUpElementUnderTest<CheckOrRadio>('checkbox-sk');

  let checkOrRadio: CheckOrRadio;

  beforeEach(() => {
    checkOrRadio = newInstance();
  });

  it('responds to click()', async () => {
    assert.isFalse(checkOrRadio.checked);

    const change = eventPromise('change');
    checkOrRadio.click();
    await change;
    assert.isTrue(checkOrRadio.checked);

    checkOrRadio.click();
    assert.isFalse(checkOrRadio.checked);
  });

  it('has a unique ID for the input and matching label for', () => {
    const input = checkOrRadio.querySelector('input')!;
    const label = checkOrRadio.querySelector('label')!;
    assert.ok(input.id);
    assert.equal(label.getAttribute('for'), input.id);
  });

  it('generates unique IDs across instances', () => {
    const other = newInstance();
    const input1 = checkOrRadio.querySelector('input')!;
    const input2 = other.querySelector('input')!;
    assert.notEqual(input1.id, input2.id);
  });
});
