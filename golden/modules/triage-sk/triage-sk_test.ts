import './index';
import {
  eventPromise,
  noEventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { LabelOrEmpty, TriageSk } from './triage-sk';
import { Label } from '../rpc_types';

describe('triage-sk', () => {
  const newInstance = setUpElementUnderTest<TriageSk>('triage-sk');

  let triageSk: TriageSk;
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
      expect(() => triageSk.value = 'hello world' as LabelOrEmpty)
        .to.throw(RangeError, 'Invalid triage-sk value: "hello world".');
    });
  });

  describe('buttons', () => {
    let changeEvent: Promise<CustomEvent<LabelOrEmpty>>;
    beforeEach(() => {
      changeEvent = eventPromise<CustomEvent<LabelOrEmpty>>('change', 100);
    });

    it('sets value to positive when clicking positive button', async () => {
      clickButton(triageSk,'positive');
      expectValueAndToggledButtonToBe(triageSk, 'positive');
      expect((await changeEvent).detail).to.equal('positive');
    });

    it('sets value to negative when clicking negative button', async () => {
      clickButton(triageSk,'negative');
      expectValueAndToggledButtonToBe(triageSk, 'negative');
      expect((await changeEvent).detail).to.equal('negative');
    });

    it('sets value to untriaged when clicking untriaged button', async () => {
      triageSk.value = 'positive'; // Untriaged by default; change value first.
      clickButton(triageSk,'untriaged');
      expectValueAndToggledButtonToBe(triageSk, 'untriaged');
      expect((await changeEvent).detail).to.equal('untriaged');
    });

    it('does not emit event "change" when clicking button for current value',
      async () => {
        const noChangeEvent = noEventPromise('change');
        clickButton(triageSk,'untriaged');
        await noChangeEvent;
      });
  });
});

const clickButton = (triageSk: TriageSk, value: Label) =>
    triageSk.querySelector<HTMLButtonElement>(`button.${value}`)!.click();

const expectValueAndToggledButtonToBe = (triageSk: TriageSk, value: LabelOrEmpty) => {
  expect(triageSk.value).to.equal(value);
  if (value === '') {
    expect(triageSk.querySelectorAll('button.selected')).to.have.length(0);
  } else {
    expect(triageSk.querySelector(`button.${value}`)!.className).to.contain('selected');
  }
};
