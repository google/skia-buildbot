import './index';
import { $, $$ } from 'common-sk/modules/dom';
import {
  eventPromise,
  noEventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('triage-sk', () => {
  const newInstance = setUpElementUnderTest('triage-sk');

  let triageSk;
  beforeEach(() => triageSk = newInstance());

  it('is untriaged by default', () => {
    expectValueAndToggledButtonToBe(triageSk, 'untriaged');
  });

  describe('"value" property setter/getter', () => {
    it('sets and gets value via property', () => {
      triageSk.value = '';
      expectValueAndToggledButtonToBe(triageSk, '');

      triageSk.value = 'positive';
      expectValueAndToggledButtonToBe(triageSk, 'positive');

      triageSk.value = 'negative';
      expectValueAndToggledButtonToBe(triageSk, 'negative');

      triageSk.value = 'untriaged';
      expectValueAndToggledButtonToBe(triageSk, 'untriaged');
    });

    it('does not emit event "change" when setting value via property',
      async () => {
        const noTriageEvent = noEventPromise('change');
        triageSk.value = 'positive';
        await noTriageEvent;
      });

    it('throws an exception upon an invalid value', () => {
      expect(() => triageSk.value = 'hello world')
        .to.throw(RangeError, 'Invalid triage-sk value: "hello world".');
    });
  });

  describe('buttons', () => {
    let changeEvent;
    beforeEach(() => { changeEvent = eventPromise('change', 100); });

    it('sets value to positive when clicking positive button', async () => {
      $$('button.positive', triageSk).click();
      expectValueAndToggledButtonToBe(triageSk, 'positive');
      expect((await changeEvent).detail).to.equal('positive');
    });

    it('sets value to negative when clicking negative button', async () => {
      $$('button.negative', triageSk).click();
      expectValueAndToggledButtonToBe(triageSk, 'negative');
      expect((await changeEvent).detail).to.equal('negative');
    });

    it('sets value to untriaged when clicking untriaged button', async () => {
      triageSk.value = 'positive'; // Untriaged by default; change value first.
      $$('button.untriaged', triageSk).click();
      expectValueAndToggledButtonToBe(triageSk, 'untriaged');
      expect((await changeEvent).detail).to.equal('untriaged');
    });

    it('does not emit event "change" when clicking button for current value',
      async () => {
        const noChangeEvent = noEventPromise('change');
        $$('button.untriaged', triageSk).click();
        await noChangeEvent;
      });
  });
});

const expectValueAndToggledButtonToBe = (triageSk, value) => {
  expect(triageSk.value).to.equal(value);
  if (value === '') {
    expect($('button.selected', triageSk)).to.have.length(0);
  } else {
    expect($$(`button.${value}`, triageSk).className).to.contain('selected');
  }
};
