import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { BulkTriageLabel, BulkTriageSk } from './bulk-triage-sk';
import { BulkTriageSkPO } from './bulk-triage-sk_po';
import { bulkTriageDeltaInfos } from './test_data';
import {
  Label, TriageDelta, TriageRequestV3, TriageResponse,
} from '../rpc_types';

describe('bulk-triage-sk', () => {
  const newInstance = setUpElementUnderTest<BulkTriageSk>('bulk-triage-sk');

  let bulkTriageSk: BulkTriageSk;
  let bulkTriageSkPO: BulkTriageSkPO;

  beforeEach(() => {
    bulkTriageSk = newInstance();
    bulkTriageSk.bulkTriageDeltaInfos = bulkTriageDeltaInfos;
    bulkTriageSkPO = new BulkTriageSkPO(bulkTriageSk);
  });

  it('does not show a changelist ID by default', async () => {
    expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.false;
  });

  it('show the changelist ID when provided', async () => {
    bulkTriageSk.changeListID = '123';
    expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.true;
    expect(await bulkTriageSkPO.getAffectedChangelistId()).to.equal(
      'This affects Changelist 123.',
    );
  });

  it('defaults to bulk-triaging to closest', async () => {
    expect(await bulkTriageSkPO.isClosestBtnSelected()).to.be.true;
  });

  it('defaults to not bulk-triaging all digests', async () => {
    expect(await bulkTriageSkPO.isTriageAllCheckboxChecked()).to.be.false;
  });

  it('shows the correct total digest count', async () => {
    expect(await bulkTriageSkPO.getTriageAllCheckboxLabel()).to.equal('Triage all 7 digests');
  });

  describe('triage button label', () => {
    describe('current page digests', () => {
      it('shows number of digests to triage as untriaged', async () => {
        await bulkTriageSkPO.clickUntriagedBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 3 digests as untriaged');
      });

      it('shows number of digests to triage as positive', async () => {
        await bulkTriageSkPO.clickPositiveBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 3 digests as positive');
      });

      it('shows number of digests to triage as negative', async () => {
        await bulkTriageSkPO.clickNegativeBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 3 digests as negative');
      });

      it('shows number of digests to triage as closest', async () => {
        await bulkTriageSkPO.clickClosestBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 3 digests as closest');
      });
    });

    describe('all digests', () => {
      beforeEach(async () => {
        await bulkTriageSkPO.clickTriageAllCheckbox();
      });

      it('shows number of digests to triage as untriaged', async () => {
        await bulkTriageSkPO.clickUntriagedBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 7 digests as untriaged');
      });

      it('shows number of digests to triage as positive', async () => {
        await bulkTriageSkPO.clickPositiveBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 7 digests as positive');
      });

      it('shows number of digests to triage as negative', async () => {
        await bulkTriageSkPO.clickNegativeBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 7 digests as negative');
      });

      it('shows number of digests to triage as closest', async () => {
        await bulkTriageSkPO.clickClosestBtn();
        expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 7 digests as closest');
      });
    });
  });

  it('emits a bulk_triage_cancelled event when the cancel button is clicked', async () => {
    const cancelEvent = eventPromise('bulk_triage_cancelled', 100);
    await bulkTriageSkPO.clickCancelBtn();
    await cancelEvent;
  });

  describe('RPC requests', () => {
    const makeTriageRequestDataForCurrentPageDigests = (label: BulkTriageLabel): TriageDelta[] => [
      {
        grouping: {
          name: 'alpha_test',
          source_type: 'animal_corpus',
        },
        digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        label_before: 'positive',
        label_after: label === 'closest' ? 'positive' : label,
      },
      {
        grouping: {
          name: 'alpha_test',
          source_type: 'animal_corpus',
        },
        digest: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        label_before: 'negative',
        label_after: label === 'closest' ? 'negative' : label,
      },
      {
        grouping: {
          name: 'beta_test',
          source_type: 'animal_corpus',
        },
        digest: 'cccccccccccccccccccccccccccccccc',
        label_before: 'untriaged',
        label_after: label === 'closest' ? 'positive' : label,
      },
    ];

    const makeTriageRequestDataForAllDigests = (label: BulkTriageLabel): TriageDelta[] => {
      const triageDeltas: TriageDelta[] = [
        {
          grouping: {
            name: 'alpha_test',
            source_type: 'animal_corpus',
          },
          digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
          label_before: 'positive',
          label_after: label === 'closest' ? 'positive' : label,
        },
        {
          grouping: {
            name: 'alpha_test',
            source_type: 'animal_corpus',
          },
          digest: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
          label_before: 'negative',
          label_after: label === 'closest' ? 'negative' : label,
        },
        {
          grouping: {
            name: 'alpha_test',
            source_type: 'animal_corpus',
          },
          digest: 'dddddddddddddddddddddddddddddddd',
          label_before: 'untriaged',
          label_after: label === 'closest' ? 'positive' : label,
        },
        {
          grouping: {
            name: 'alpha_test',
            source_type: 'plant_corpus',
          },
          digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
          label_before: 'untriaged',
          label_after: label === 'closest' ? 'negative' : label,
        },
        {
          grouping: {
            name: 'beta_test',
            source_type: 'animal_corpus',
          },
          digest: 'cccccccccccccccccccccccccccccccc',
          label_before: 'untriaged',
          label_after: label === 'closest' ? 'positive' : label,
        },
        {
          grouping: {
            name: 'beta_test',
            source_type: 'animal_corpus',
          },
          digest: 'dddddddddddddddddddddddddddddddd',
          label_before: 'untriaged',
          label_after: label === 'closest' ? 'negative' : label,
        },
      ];
      if (label !== 'closest') {
        // This one does not have a closest diff.
        triageDeltas.push({
          grouping: {
            name: 'gamma_test',
            source_type: 'animal_corpus',
          },
          digest: 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
          label_before: 'positive',
          label_after: label,
        });
      }
      return triageDeltas;
    };

    const test = async (expectedRequest: TriageRequestV3) => {
      const response: TriageResponse = { status: 'ok' };
      fetchMock.post({
        url: '/json/v3/triage',
        body: expectedRequest,
      }, {
        status: 200,
        body: response,
      });

      const finishedPromise = eventPromise('bulk_triage_finished');
      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    };

    afterEach(() => {
      expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
      fetchMock.reset();
    });

    describe('current page digests', () => {
      describe('at head', () => {
        const makeTriageRequest = (label: BulkTriageLabel): TriageRequestV3 => ({
          deltas: makeTriageRequestDataForCurrentPageDigests(label),
        });

        it('triages as untriaged', async () => {
          await bulkTriageSkPO.clickUntriagedBtn();
          await test(makeTriageRequest('untriaged'));
        });

        it('triages as positive', async () => {
          await bulkTriageSkPO.clickPositiveBtn();
          await test(makeTriageRequest('positive'));
        });

        it('triages as negative', async () => {
          await bulkTriageSkPO.clickNegativeBtn();
          await test(makeTriageRequest('negative'));
        });

        it('triages as closest', async () => {
          await bulkTriageSkPO.clickClosestBtn();
          await test(makeTriageRequest('closest'));
        });
      });

      describe('at CL', () => {
        const makeTriageRequest = (label: BulkTriageLabel): TriageRequestV3 => ({
          deltas: makeTriageRequestDataForCurrentPageDigests(label),
          changelist_id: 'someCL',
          crs: 'gerrit',
        });

        beforeEach(async () => {
          bulkTriageSk.changeListID = 'someCL';
          bulkTriageSk.crs = 'gerrit';
        });

        it('triages as untriaged', async () => {
          await bulkTriageSkPO.clickUntriagedBtn();
          await test(makeTriageRequest('untriaged'));
        });

        it('triages as positive', async () => {
          await bulkTriageSkPO.clickPositiveBtn();
          await test(makeTriageRequest('positive'));
        });

        it('triages as negative', async () => {
          await bulkTriageSkPO.clickNegativeBtn();
          await test(makeTriageRequest('negative'));
        });

        it('triages as closest', async () => {
          await bulkTriageSkPO.clickClosestBtn();
          await test(makeTriageRequest('closest'));
        });
      });
    });

    describe('all digests', () => {
      beforeEach(async () => {
        await bulkTriageSkPO.clickTriageAllCheckbox();
      });

      describe('at head', () => {
        const makeTriageRequest = (label: BulkTriageLabel): TriageRequestV3 => ({
          deltas: makeTriageRequestDataForAllDigests(label),
        });

        it('triages as untriaged', async () => {
          await bulkTriageSkPO.clickUntriagedBtn();
          await test(makeTriageRequest('untriaged'));
        });

        it('triages as positive', async () => {
          await bulkTriageSkPO.clickPositiveBtn();
          await test(makeTriageRequest('positive'));
        });

        it('triages as negative', async () => {
          await bulkTriageSkPO.clickNegativeBtn();
          await test(makeTriageRequest('negative'));
        });

        it('triages as closest', async () => {
          await bulkTriageSkPO.clickClosestBtn();
          await test(makeTriageRequest('closest'));
        });
      });

      describe('at CL', () => {
        const makeTriageRequest = (label: BulkTriageLabel): TriageRequestV3 => ({
          deltas: makeTriageRequestDataForAllDigests(label),
          changelist_id: 'someCL',
          crs: 'gerrit',
        });

        beforeEach(async () => {
          bulkTriageSk.changeListID = 'someCL';
          bulkTriageSk.crs = 'gerrit';
        });

        it('triages as untriaged', async () => {
          await bulkTriageSkPO.clickUntriagedBtn();
          await test(makeTriageRequest('untriaged'));
        });

        it('triages as positive', async () => {
          await bulkTriageSkPO.clickPositiveBtn();
          await test(makeTriageRequest('positive'));
        });

        it('triages as negative', async () => {
          await bulkTriageSkPO.clickNegativeBtn();
          await test(makeTriageRequest('negative'));
        });

        it('triages as closest', async () => {
          await bulkTriageSkPO.clickClosestBtn();
          await test(makeTriageRequest('closest'));
        });
      });
    });
  });
});
