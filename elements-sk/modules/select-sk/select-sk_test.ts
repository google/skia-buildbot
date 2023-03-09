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
import { SelectSk } from './select-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('select-sk', () => {
  const newInstance = setUpElementUnderTest<SelectSk>('select-sk');

  interface InstantiationOptions {
    disabled: boolean;
    itemIds: string[];
    selectedItemId: string;
  }

  const newInstanceWithOpts = async (opts: Partial<InstantiationOptions>): Promise<SelectSk> => {
    const instance = newInstance((instance: SelectSk) => {
      if (opts.disabled) instance.setAttribute('disabled', '');
      (opts.itemIds || []).forEach((id) => {
        const item = document.createElement('div');
        item.id = id;
        if (opts.selectedItemId && opts.selectedItemId === id) item.setAttribute('selected', '');
        instance.appendChild(item);
      });
    });
    await Promise.resolve(); // Give the mutation observer a chance to fire.
    return instance;
  };

  describe('selection property', () => {
    it('has a default value', () => {
      const selectSk = newInstance();
      assert.equal(-1, (selectSk as any)._selection);
      assert.equal(-1, selectSk.selection);
    });

    it('changes based on children', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemId: 'b',
      });
      assert.equal(1, (selectSk as any)._selection);
      assert.equal(1, selectSk.selection);
    });

    it('can go back to -1', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemId: 'b',
      });
      selectSk.selection = -1;
      assert.equal(-1, selectSk.selection);
      assert.isFalse(selectSk.querySelector('#b')!.hasAttribute('selected'));
    });

    it('parses strings', async () => {
      const selectSk = await newInstanceWithOpts({ itemIds: ['a', 'b'] });
      selectSk.selection = '1';
      assert.equal(1, +selectSk.selection);
      assert.isTrue(selectSk.querySelector('#b')!.hasAttribute('selected'));
    });

    it('treats null and undefined and out of range as -1', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemId: 'b',
      });
      selectSk.selection = null;
      assert.equal(-1, selectSk.selection);
      assert.isFalse(selectSk.querySelector('#b')!.hasAttribute('selected'));

      selectSk.selection = 0;
      assert.equal(0, selectSk.selection);
      assert.isTrue(selectSk.querySelector('#a')!.hasAttribute('selected'));

      selectSk.selection = undefined;
      assert.equal(-1, selectSk.selection);

      selectSk.selection = 10;
      assert.equal(-1, selectSk.selection);

      selectSk.selection = -3;
      assert.equal(-1, selectSk.selection);
    });

    it('changes selected attributes on children', async () => {
      const selectSk = await newInstanceWithOpts({ itemIds: ['a', 'b'] });
      const a = selectSk.querySelector('#a')!;
      const b = selectSk.querySelector('#b')!;
      selectSk.selection = 0;
      assert.equal(0, selectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      selectSk.selection = 1;
      assert.equal(1, selectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isTrue(b.hasAttribute('selected'));
    });

    it('stays fixed when disabled', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['a', 'b'],
        selectedItemId: 'b',
      });
      assert.equal(1, (selectSk as any)._selection);
      assert.equal(1, selectSk.selection);
      assert.equal('0', selectSk.getAttribute('tabindex'));
      selectSk.disabled = true;
      selectSk.selection = 0;
      assert.equal(1, (selectSk as any)._selection);
      assert.equal(1, selectSk.selection);
      assert.equal(false, selectSk.hasAttribute('tabindex'));
    });

    it('gets updated when select-sk is re-enabled', async () => {
      const selectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['a', 'b'],
        selectedItemId: 'b',
      });
      assert.equal(-1, (selectSk as any)._selection);
      assert.equal(-1, selectSk.selection);
      selectSk.disabled = false;
      assert.equal(1, (selectSk as any)._selection);
      assert.equal(1, selectSk.selection);
      assert.isFalse(selectSk.hasAttribute('disabled'));
    });
  }); // end describe('selected property')

  describe('click', () => {
    it('changes selection', async () => {
      const selectSk = await newInstanceWithOpts({ itemIds: ['a', 'b'] });
      assert.equal('listbox', selectSk.getAttribute('role'));
      const a = selectSk.querySelector<HTMLDivElement>('#a')!;
      const b = selectSk.querySelector<HTMLDivElement>('#b')!;
      a.click();
      assert.equal(0, selectSk.selection);
      assert.isTrue(a.hasAttribute('selected'));
      assert.equal('true', a.getAttribute('aria-selected'));
      assert.equal('option', a.getAttribute('role'));
      assert.isFalse(b.hasAttribute('selected'));
      assert.equal('false', b.getAttribute('aria-selected'));
      assert.equal('option', b.getAttribute('role'));
      b.click();
      assert.equal(1, selectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.equal('false', a.getAttribute('aria-selected'));
      assert.isTrue(b.hasAttribute('selected'));
      assert.equal('true', b.getAttribute('aria-selected'));
    });

    it('ignores clicks when disabled', async () => {
      const selectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['a', 'b'],
      });
      const a = selectSk.querySelector<HTMLDivElement>('#a')!;
      const b = selectSk.querySelector<HTMLDivElement>('#b')!;
      a.click();
      assert.equal(-1, selectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
      b.click();
      assert.equal(-1, selectSk.selection);
      assert.isFalse(a.hasAttribute('selected'));
      assert.isFalse(b.hasAttribute('selected'));
    });
  }); // end describe('click')

  describe('inserting new children', () => {
    it('updates selection property', async () => {
      const selectSk = await newInstanceWithOpts({ itemIds: ['', '', ''] });
      assert.equal(-1, selectSk.selection);
      let div = document.createElement('div');
      div.setAttribute('selected', '');
      selectSk.appendChild(div);
      div = document.createElement('div');
      selectSk.appendChild(div);
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.equal(3, selectSk.selection);
    });

    it('does not check children when disabled', async () => {
      const selectSk = await newInstanceWithOpts({
        disabled: true,
        itemIds: ['', '', ''],
      });
      assert.equal(-1, selectSk.selection);
      let div = document.createElement('div');
      div.setAttribute('selected', '');
      selectSk.appendChild(div);
      div = document.createElement('div');
      selectSk.appendChild(div);
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.equal(-1, selectSk.selection);
    });
  }); // end describe('inserting new children')

  describe('mutation of child selected attribute', () => {
    it('does update selection', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['', 'd1', 'd2'],
        selectedItemId: 'd2',
      });
      assert.equal(2, selectSk.selection);
      selectSk.querySelector('#d2')!.removeAttribute('selected');
      selectSk.querySelector('#d1')!.setAttribute('selected', '');
      // Need to do the check post microtask so the mutation observer gets a
      // chance to fire.
      await Promise.resolve();
      assert.equal(1, selectSk.selection);
    });
  }); // end describe('mutation of child selected attribute'

  describe('keyboard navigation', () => {
    it('follows arrow keys', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['', '', 'd2'],
        selectedItemId: 'd2',
      });
      assert.equal(2, selectSk.selection);
      (selectSk as any)._onKeyDown(new KeyboardEvent('keydown', { key: 'ArrowUp' }));
      assert.equal(1, selectSk.selection);
      (selectSk as any)._onKeyDown(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
      assert.equal(2, selectSk.selection);
      (selectSk as any)._onKeyDown(new KeyboardEvent('keydown', { key: 'Home' }));
      assert.equal(0, selectSk.selection);
      (selectSk as any)._onKeyDown(new KeyboardEvent('keydown', { key: 'End' }));
      assert.equal(2, selectSk.selection);
      // Don't wrap around.
      (selectSk as any)._onKeyDown(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
      assert.equal(2, selectSk.selection);
    });
  }); // end describe('keyboard navigation')

  describe('focus', () => {
    it('drops focus when disabled', async () => {
      const selectSk = await newInstanceWithOpts({
        itemIds: ['', '', 'd2'],
        selectedItemId: 'd2',
      }); selectSk.focus();
      assert.equal(selectSk, document.activeElement);
      selectSk.disabled = true;
      assert.notEqual(selectSk, document.activeElement);
    });
  }); // end describe('focus')
});
