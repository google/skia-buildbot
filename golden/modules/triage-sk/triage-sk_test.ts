import './index';
import { expect } from 'chai';
import {
  eventPromise,
  noEventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { TriageSk } from './triage-sk';
import { TriageSkPO } from './triage-sk_po';
import { Label } from '../rpc_types';

describe('triage-sk', () => {
  const newInstance = setUpElementUnderTest<TriageSk>('triage-sk');

  let triageSk: TriageSk;
  let triageSkPO: TriageSkPO;

  beforeEach(() => {
    triageSk = newInstance();
    triageSkPO = new TriageSkPO(triageSk);
  });

  it('is untriaged by default', async () => {
    await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'untriaged');
  });

  describe('"value" property setter/getter', () => {
    it('sets and gets value via property', async () => {
      triageSk.value = 'positive';
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'positive');

      triageSk.value = 'negative';
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'negative');

      triageSk.value = 'untriaged';
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'untriaged');
    });

    it('does not emit event "change" when setting value via property',
      async () => {
        const noTriageEvent = noEventPromise('change');
        triageSk.value = 'positive';
        await noTriageEvent;
      });
  });

  describe('buttons', () => {
    it('sets value to positive when clicking positive button', async () => {
      const changeEvent = eventPromise<CustomEvent<Label>>('change', 100);
      await triageSkPO.clickButton('positive');
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'positive');
      expect((await changeEvent).detail).to.equal('positive');
    });

    it('sets value to negative when clicking negative button', async () => {
      const changeEvent = eventPromise<CustomEvent<Label>>('change', 100);
      await triageSkPO.clickButton('negative');
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'negative');
      expect((await changeEvent).detail).to.equal('negative');
    });

    it('sets value to untriaged when clicking untriaged button', async () => {
      const changeEvent = eventPromise<CustomEvent<Label>>('change', 100);
      triageSk.value = 'positive'; // Untriaged by default; change value first.
      await triageSkPO.clickButton('untriaged');
      await expectValueAndToggledButtonToBe(triageSk, triageSkPO, 'untriaged');
      expect((await changeEvent).detail).to.equal('untriaged');
    });

    it('does not emit event "change" when clicking button for current value',
      async () => {
        const noChangeEvent = noEventPromise('change');
        await triageSkPO.clickButton('untriaged');
        await noChangeEvent;
      });
  });
});

const expectValueAndToggledButtonToBe = async (triageSk: TriageSk, triageSkPO: TriageSkPO, value: Label) => {
  expect(triageSk.value).to.equal(value);
  expect(await triageSkPO.getLabel()).to.equal(value);
};
