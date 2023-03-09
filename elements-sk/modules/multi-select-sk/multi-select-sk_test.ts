// Copyright 2018 Google LLC
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
import { MultiSelectSk } from './multi-select-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('multi-select-sk', () => {
  const newInstance = setUpElementUnderTest<MultiSelectSk>('multi-select-sk');

  interface InstantiationOptions {
    disabled: boolean;
    itemIds: string[];
    selectedItemIds: string[];
  }

  const newInstanceWithOpts = async (opts: Partial<InstantiationOptions>): Promise<MultiSelectSk> => {
    const instance = newInstance((instance: MultiSelectSk) => {
      if (opts.disabled) instance.setAttribute('disabled', '');
      (opts.itemIds || []).forEach((id) => {
        const item = document.createElement('div');
        item.id = id;
        if (opts.selectedItemIds?.includes(id)) item.setAttribute('selected', '');
        instance.appendChild(item);
      });
    });
    await Promise.resolve(); // Give the mutation observer a chance to fire.
    return instance;
  };

  describe('selection property', () => {
    it('has a default value', () => {
      const multiSelectSk = newInstance();
      assert.deepEqual([], (multiSelectSk as any)._selection);
      assert.deepEqual([], multiSelectSk.selection);
    });

    it('changes based on children', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemIds: ['b'],
      });
      assert.deepEqual([1], (multiSelectSk as any)._selection);
      assert.deepEqual([1], multiSelectSk.selection);
    });

    it('can go back to []', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemIds: ['b'],
      });
      multiSelectSk.selection = [];
      assert.deepEqual([], multiSelectSk.selection);
      assert.isFalse(multiSelectSk.querySelector('#b')!.hasAttribute('selected'));
    });

    it('changes the selected attributes on the children', async () => {
      const multiSelectSk = await newInstanceWithOpts({ itemIds: ['a', 'b'] });
      const a = multiSelectSk.querySelector('#a')!;
      const b = multiSelectSk.querySelector('#b')!;
      multiSelectSk.selection = [0];
      assert.deepEqual([0], multiSelectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      multiSelectSk.selection = [0, 1];

      assert.deepEqual([0, 1], multiSelectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isTrue(b.hasAttribute('selected'));
    });

    it('is stays fixed when disabled', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemIds: ['b'],
      });
      assert.deepEqual([1], (multiSelectSk as any)._selection);
      assert.deepEqual([1], multiSelectSk.selection);
      multiSelectSk.disabled = true;
      multiSelectSk.selection = [0];
      assert.deepEqual([1], (multiSelectSk as any)._selection);
      assert.deepEqual([1], multiSelectSk.selection);
      assert.isTrue(multiSelectSk.hasAttribute('disabled'));
    });

    it('gets updated when re-enabled', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['a', 'b'],
        selectedItemIds: ['b'],
      });
      multiSelectSk.disabled = true;
      assert.deepEqual([], (multiSelectSk as any)._selection);
      assert.deepEqual([], multiSelectSk.selection);
      multiSelectSk.disabled = false;
      assert.deepEqual([1], (multiSelectSk as any)._selection);
      assert.deepEqual([1], multiSelectSk.selection);
      assert.isFalse(multiSelectSk.hasAttribute('disabled'));
    });

    it('is always sorted when read', async () => {
      const multiSelectSk = await newInstanceWithOpts({ itemIds: ['', '', '', '', '', ''] });
      multiSelectSk.selection = [5, 4, 0, 2];
      assert.deepEqual([0, 2, 4, 5], multiSelectSk.selection);
    });
  }); // end describe('selection property')

  describe('click', () => {
    it('changes selection in an additive fashion', async () => {
      const multiSelectSk = await newInstanceWithOpts({ itemIds: ['a', 'b', 'c'] });
      const a = multiSelectSk.querySelector<HTMLDivElement>('#a')!;
      const b = multiSelectSk.querySelector<HTMLDivElement>('#b')!;
      const c = multiSelectSk.querySelector<HTMLDivElement>('#c')!;
      a.click();
      assert.deepEqual([0], multiSelectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
      b.click();
      assert.deepEqual([0, 1], multiSelectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isTrue(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
      // unselect
      b.click();
      assert.deepEqual([0], multiSelectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
    });

    it('ignores clicks when disabled', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['a', 'b', 'c'],
      });
      const a = multiSelectSk.querySelector<HTMLDivElement>('#a')!;
      const b = multiSelectSk.querySelector<HTMLDivElement>('#b')!;
      const c = multiSelectSk.querySelector<HTMLDivElement>('#c')!;
      a.click();
      assert.deepEqual([], multiSelectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
      b.click();
      assert.deepEqual([], multiSelectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
      // unselect
      b.click();
      assert.deepEqual([], multiSelectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.isFalse(c.hasAttribute('selected'));
    });
  }); // end describe('click')

  describe('addition of children', () => {
    it('updates selection when a selected child is added', async () => {
      const multiSelectSk = await newInstanceWithOpts({ itemIds: ['', '', ''] });
      assert.deepEqual([], multiSelectSk.selection);
      let div = document.createElement('div');
      div.setAttribute('selected', '');
      multiSelectSk.appendChild(div);
      div = document.createElement('div');
      multiSelectSk.appendChild(div);
      div = document.createElement('div');
      div.setAttribute('selected', '');
      multiSelectSk.appendChild(div);
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.deepEqual([3, 5], multiSelectSk.selection);
    });

    it('does not check children when disabled', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['', '', ''],
      });
      assert.deepEqual([], multiSelectSk.selection);
      let div = document.createElement('div');
      div.setAttribute('selected', '');
      multiSelectSk.appendChild(div);
      div = document.createElement('div');
      multiSelectSk.appendChild(div);
      div = document.createElement('div');
      div.setAttribute('selected', '');
      multiSelectSk.appendChild(div);
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.deepEqual([], multiSelectSk.selection);
    });
  }); // end describe('addition of children')

  describe('mutation of child selected attribute', () => {
    it('does update selection', async () => {
      const multiSelectSk = await newInstanceWithOpts({
        itemIds: ['', 'd1', 'd2'],
        selectedItemIds: ['d2'],
      });
      assert.deepEqual([2], multiSelectSk.selection);
      multiSelectSk.querySelector('#d2')!.removeAttribute('selected');
      multiSelectSk.querySelector('#d1')!.setAttribute('selected', '');
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.deepEqual([1], multiSelectSk.selection);
    });
  }); // end describe('mutation of child selected attribute')
});
