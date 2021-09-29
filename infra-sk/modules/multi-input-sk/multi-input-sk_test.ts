import './index';
import { expect } from 'chai';
import { $ } from 'common-sk/modules/dom';
import { MultiInputSk } from './multi-input-sk';

import { setUpElementUnderTest } from '../test_util';

describe('multi-input-sk', () => {
  const newInstance = setUpElementUnderTest<MultiInputSk>('multi-input-sk');

  let ele: MultiInputSk;
  beforeEach(() => {
    ele = newInstance();
  });

  describe('input behavior', () => {
    it('gets and sets values', () => {
      expect(ele.values.length).to.equal(0);
      expect($('.input-item', ele).length).to.equal(0);
      ele.values = ['abc', '123'];
      expect(ele.values.length).to.equal(2);
      expect(ele.values[0]).to.equal('abc');
      expect(ele.values[1]).to.equal('123');
      const inputItems = $<HTMLDivElement>('.input-item', ele);
      expect(inputItems.length).to.equal(2);
      expect(inputItems[0].innerText).to.equal('abc ');
      expect(inputItems[1].innerText).to.equal('123 ');
    });
  });
});
