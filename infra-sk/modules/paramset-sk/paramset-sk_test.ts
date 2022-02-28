import './index';

import { expect } from 'chai';
import { ParamSet } from 'common-sk/modules/query';
import { setUpElementUnderTest, eventPromise, noEventPromise } from '../test_util';
import { ParamSetSk, ParamSetSkClickEventDetail } from './paramset-sk';
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
        a: 'hello', b: '1', c: 'X', d: 'IV', e: 'alpha',
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
        await paramSetSkPO.clickValue({ paramSetIndex: 0, key: 'a', value: 'hello' });
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
        const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>('paramset-key-value-click');
        await paramSetSkPO.clickValue({ paramSetIndex: 0, key: 'a', value: 'hello' });
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
        const expectedDetail: ParamSetSkClickEventDetail = { key: 'a', ctrl: false };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });

      it('emits an event when clicking a value', async () => {
        const event = eventPromise<CustomEvent<ParamSetSkClickEventDetail>>('paramset-key-value-click');
        await paramSetSkPO.clickValue({ paramSetIndex: 0, key: 'a', value: 'hello' });
        const expectedDetail: ParamSetSkClickEventDetail = { key: 'a', value: 'hello', ctrl: false };
        expect((await event).detail).to.deep.equal(expectedDetail);
      });
    });
  });
});
