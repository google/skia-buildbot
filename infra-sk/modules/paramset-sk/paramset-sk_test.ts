import './index';

import { expect } from 'chai';
import { assert } from 'chai';
import { ParamSet } from '../query';
import { setUpElementUnderTest, eventPromise, noEventPromise } from '../test_util';
import {
  ParamSetSk,
  ParamSetSkClickEventDetail,
  ParamSetSkKeyCheckboxClickEventDetail,
  ParamSetSkPlusClickEventDetail,
  ParamSetSkRemoveClickEventDetail,
} from './paramset-sk';
import { ParamSetSkPO, ParamSetKeyValueTuple } from './paramset-sk_po';

const paramSet1: ParamSet = {
  a: ['hello', 'world'],
  b: ['1', '2', '3'],
  c: ['W', 'X', 'Y'],
  d: ['I', 'II', 'III'],
};
const paramSet2: ParamSet = {
  b: ['2', '3', '4'],
  c: ['X', 'Y', 'Z'],
  d: ['II', 'III', 'IV'],
  e: ['alpha'],
};

const title1 = 'foo';
const title2 = 'bar';

describe('paramset-sk with checkbox', () => {
  const newInstance = setUpElementUnderTest<ParamSetSk>('paramset-sk');

  let paramSetSk: ParamSetSk;
  let paramSetSkPO: ParamSetSkPO;

  beforeEach(() => {
    paramSetSk = newInstance();
    paramSetSk.checkbox_values = true;
    paramSetSkPO = new ParamSetSkPO(paramSetSk);
  });

  describe('key checkboxes', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1];
      paramSetSk.titles = [title1];
    });

    it('emits an event when the checkbox for a key is checked', async () => {
      const event = eventPromise<CustomEvent<ParamSetSkKeyCheckboxClickEventDetail>>(
        'paramset-key-checkbox-click'
      );
      await paramSetSkPO.clickKeyCheckbox('a');
      const expectedDetail: ParamSetSkKeyCheckboxClickEventDetail = {
        key: 'a',
        values: ['world'], // the first value "hello" will not be emitted.
        selected: false,
      };
      expect((await event).detail).to.deep.equal(expectedDetail);

      // Now let's click the checkbox again.
      const event2 = eventPromise<CustomEvent<ParamSetSkKeyCheckboxClickEventDetail>>(
        'paramset-key-checkbox-click'
      );
      await paramSetSkPO.clickKeyCheckbox('a');
      const expectedDetail2: ParamSetSkKeyCheckboxClickEventDetail = {
        key: 'a',
        values: ['hello', 'world'], // all values should be there.
        selected: true,
      };
      expect((await event2).detail).to.deep.equal(expectedDetail2);
    });
  });
});

describe('paramset-sk', () => {
  const newInstance = setUpElementUnderTest<ParamSetSk>('paramset-sk');

  let paramSetSk: ParamSetSk;
  let paramSetSkPO: ParamSetSkPO;

  beforeEach(() => {
    paramSetSk = newInstance();
    paramSetSkPO = new ParamSetSkPO(paramSetSk);
  });

  describe('one single ParamSet', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1];
      paramSetSk.titles = [title1];
    });

    it('should display the ParamSet', async () => {
      expect(await paramSetSkPO.getParamSets()).to.deep.equal([paramSet1]);
    });

    it('should display the title', async () => {
      expect(await paramSetSkPO.getTitles()).to.deep.equal([title1]);
    });
  });

  describe('multiple ParamSets', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1, paramSet2];
      paramSetSk.titles = [title1, title2];
    });

    it('should display the ParamSet', async () => {
      expect(await paramSetSkPO.getParamSets()).to.deep.equal([paramSet1, paramSet2]);
    });

    it('should display the title', async () => {
      expect(await paramSetSkPO.getTitles()).to.deep.equal([title1, title2]);
    });
  });

  describe('titles', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1, paramSet2];
    });

    it('should not be visible if none are provided', async () => {
      expect(await paramSetSkPO.getTitles()).to.deep.equal(['', '']);
    });

    it('should not be visible there are fewer titles than there are ParamSets', async () => {
      paramSetSk.titles = [title1];
      expect(await paramSetSkPO.getTitles()).to.deep.equal(['', '']);
    });

    it('should not be visible there are more titles than there are ParamSets', async () => {
      paramSetSk.titles = [title1, title2, 'superfluous title'];
      expect(await paramSetSkPO.getTitles()).to.deep.equal(['', '']);
    });
  });

  describe('highlighed values', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1, paramSet2];
    });

    it('should not highlight anything by default', async () => {
      expect(await paramSetSkPO.getHighlightedValues()).to.deep.equal([]);
    });

    it('should highlight values', async () => {
      paramSetSk.highlight = {
        a: 'hello',
        b: '1',
        c: 'X',
        d: 'IV',
        e: 'alpha',
      };

      const expected: ParamSetKeyValueTuple[] = [
        { paramSetIndex: 0, key: 'a', value: 'hello' },
        { paramSetIndex: 0, key: 'b', value: '1' },
        { paramSetIndex: 0, key: 'c', value: 'X' },
        { paramSetIndex: 1, key: 'c', value: 'X' },
        { paramSetIndex: 1, key: 'd', value: 'IV' },
        { paramSetIndex: 1, key: 'e', value: 'alpha' },
      ];
      expect(await paramSetSkPO.getHighlightedValues()).to.deep.equal(expected);
    });
  });

  describe('clicks', () => {
    beforeEach(() => {
      paramSetSk.paramsets = [paramSet1, paramSet2];
      paramSetSk.titles = [title1, title2];
    });

    describe('not clickable', () => {
      it('does not emit an event when clicking a key', async () => {
        const noEvent = noEventPromise('paramset-key-click');
        await paramSetSkPO.clickKey('a');
        await noEvent;
      });

      it('does not emit an event when clicking a value', async () => {
        const noEvent = noEventPromise('paramset-key-value-click');
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: 'a',
          value: 'hello',
        });
        await noEvent;
      });
    });

    describe('only values are clickable', () => {
      beforeEach(() => {
        paramSetSk.clickable_values = true;
      });

      it('does not emit an event when clicking a key', async () => {
        const noEvent = noEventPromise('paramset-key-click');
        await paramSetSkPO.clickKey('a');
        await noEvent;
      });

      it('emits an event when clicking a value', async () => {
        const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>(
          'paramset-key-value-click'
        );
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: 'a',
          value: 'hello',
        });
        const expectedDetail: ParamSetSkClickEventDetail = {
          ctrl: false,
          key: 'a',
          value: 'hello',
        };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });
    });

    describe('clicking keys and values', () => {
      beforeEach(() => {
        paramSetSk.clickable = true;
      });

      it('emits an event when clicking a key', async () => {
        const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>('paramset-key-click');
        await paramSetSkPO.clickKey('a');
        const expectedDetail: ParamSetSkClickEventDetail = {
          key: 'a',
          ctrl: false,
        };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });

      it('emits an event when clicking a value', async () => {
        const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>(
          'paramset-key-value-click'
        );
        await paramSetSkPO.clickValue({
          paramSetIndex: 0,
          key: 'a',
          value: 'hello',
        });
        const expectedDetail: ParamSetSkClickEventDetail = {
          key: 'a',
          value: 'hello',
          ctrl: false,
        };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });
    });

    describe('clicking plus', () => {
      beforeEach(() => {
        paramSetSk.clickable_plus = true;
        paramSetSk.clickable_values = true;
      });

      it('emits an event when clicking the plus', async () => {
        const event = eventPromise<CustomEvent<ParamSetSkPlusClickEventDetail>>('plus-click');
        await paramSetSkPO.clickPlus('a');
        const expectedDetail: ParamSetSkPlusClickEventDetail = {
          key: 'a',
          values: paramSet1.a,
        };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });
    });
  });

  describe('Removable values', () => {
    beforeEach(() => {
      paramSetSk.removable_values = true;
      paramSetSk.paramsets = [paramSet1];
    });

    it('generates the relevant event when the remove button is clicked', async () => {
      const key = 'a';
      const val = 'hello';
      const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>(
        'paramset-value-remove-click'
      );
      await paramSetSkPO.clickValue({
        paramSetIndex: 0,
        key: key,
        value: val,
      });

      paramSetSkPO.removeSelectedValue(key, val);
      const expectedDetail: ParamSetSkRemoveClickEventDetail = {
        key: key,
        value: val,
      };
      expect((await event).detail).to.deep.equal(expectedDetail);
      assert.deepEqual(paramSet1[key], ['world']);
    });
  });
});
