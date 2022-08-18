import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { deepCopy } from 'common-sk/modules/object';
import { ErrorSkEventDetail } from 'elements-sk/errorMessage';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { twoHundredCommits, typicalDetails } from './test_data';
import { DigestDetailsSk } from './digest-details-sk';
import { DigestDetailsSkPO } from './digest-details-sk_po';
import { Label, TriageRequestV3, TriageResponse } from '../rpc_types';
import { groupingsResponse } from '../search-page-sk/demo_data';

describe('digest-details-sk', () => {
  const newInstance = setUpElementUnderTest<DigestDetailsSk>('digest-details-sk');

  let digestDetailsSk: DigestDetailsSk;
  let digestDetailsSkPO: DigestDetailsSkPO;

  const regularNow = Date.now;

  beforeEach(() => {
    digestDetailsSk = newInstance();
    digestDetailsSkPO = new DigestDetailsSkPO(digestDetailsSk);

    Date.now = () => Date.parse('2020-01-01T00:00:00Z');
  });

  afterEach(() => {
    Date.now = regularNow;
  });

  describe('layout with positive and negative references', () => {
    beforeEach(() => {
      digestDetailsSk.groupings = deepCopy(groupingsResponse);
      digestDetailsSk.details = deepCopy(typicalDetails);
      digestDetailsSk.commits = deepCopy(twoHundredCommits);
    });

    it('shows the test name', async () => {
      expect(await digestDetailsSkPO.getTestName())
        .to.equal('Test: dots-legend-sk_too-many-digests');
    });

    it('has a link to the cluster view', async () => {
      expect(await digestDetailsSkPO.getClusterHref()).to.equal(
        '/cluster?corpus=infra&grouping=dots-legend-sk_too-many-digests&include_ignored=false'
        + '&left_filter=&max_rgba=0&min_rgba=0&negative=true&not_at_head=true&positive=true'
        + '&reference_image_required=false&right_filter=&sort=descending&untriaged=true',
      );
    });

    it('shows shows both digests', async () => {
      expect(await digestDetailsSkPO.getLeftDigest())
        .to.equal('Left: 6246b773851984c726cb2e1cb13510c2');
      expect(await digestDetailsSkPO.getRightDigest())
        .to.equal('Right: 99c58c7002073346ff55f446d47d6311');
    });

    it('shows the metrics and the link to the diff page', async () => {
      expect(await digestDetailsSkPO.getDiffPageLink()).to.equal(
        '/diff?test=dots-legend-sk_too-many-digests'
            + '&left=6246b773851984c726cb2e1cb13510c2&right=99c58c7002073346ff55f446d47d6311',
      );

      expect(await digestDetailsSkPO.getMetrics()).to.deep.equal([
        'Diff metric: 0.083',
        'Diff %: 0.22',
        'Pixels: 3766',
        'Max RGBA: [9,9,9,0]',
      ]);

      expect(await digestDetailsSkPO.isSizeWarningVisible()).to.be.false;
    });

    it('has a triage button and shows the triage history', async () => {
      expect(await digestDetailsSkPO.triageSkPO.getLabel()).to.equal('positive');
      expect(await digestDetailsSkPO.getTriageHistory()).to.equal('8w ago by user1@');
    });

    it('has an image-compare-sk with the right values', async () => {
      expect(await digestDetailsSkPO.imageCompareSkPO.getImageCaptionTexts()).to.deep.equal([
        '6246b7738519...',
        'Closest Positive',
      ]);
      expect(await digestDetailsSkPO.imageCompareSkPO.getImageCaptionHrefs()).to.deep.equal([
        '/detail?test=dots-legend-sk_too-many-digests&digest=6246b773851984c726cb2e1cb13510c2',
        '/detail?test=dots-legend-sk_too-many-digests&digest=99c58c7002073346ff55f446d47d6311',
      ]);
      expect(await digestDetailsSkPO.isClosestImageIsNegativeWarningVisible()).to.be.false;
    });

    it('changes the reference image when the toggle button is clicked', async () => {
      await digestDetailsSkPO.clickToggleReferenceBtn();

      expect(await digestDetailsSkPO.imageCompareSkPO.getImageCaptionTexts()).to.deep.equal([
        '6246b7738519...',
        'Closest Negative',
      ]);
      expect(await digestDetailsSkPO.imageCompareSkPO.getImageCaptionHrefs()).to.deep.equal([
        '/detail?test=dots-legend-sk_too-many-digests&digest=6246b773851984c726cb2e1cb13510c2',
        '/detail?test=dots-legend-sk_too-many-digests&digest=ec3b8f27397d99581e06eaa46d6d5837',
      ]);
      expect(await digestDetailsSkPO.isClosestImageIsNegativeWarningVisible()).to.be.true;
    });

    it('emits a "triage" event when a triage button is clicked', async () => {
      // Triage as negative.
      let triageEventPromise = eventPromise<CustomEvent<Label>>('triage');
      await digestDetailsSkPO.triageSkPO.clickButton('negative');
      expect((await triageEventPromise).detail).to.equal('negative');

      // Triage as positive.
      triageEventPromise = eventPromise('triage');
      await digestDetailsSkPO.triageSkPO.clickButton('positive');
      expect((await triageEventPromise).detail).to.equal('positive');

      // Triage as untriaged.
      triageEventPromise = eventPromise('triage');
      await digestDetailsSkPO.triageSkPO.clickButton('untriaged');
      expect((await triageEventPromise).detail).to.equal('untriaged');
    });

    describe('RPC requests', () => {
      afterEach(() => {
        expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
        fetchMock.reset();
      });

      const triageRequest: TriageRequestV3 = {
        deltas: [
          {
            grouping: {
              source_type: 'infra',
              name: 'dots-legend-sk_too-many-digests',
            },
            digest: '6246b773851984c726cb2e1cb13510c2',
            label_before: 'positive',
            label_after: 'negative',
          },
        ],
      };

      it('POSTs to the RPC endpoint when triage button clicked', async () => {
        const triageResponse: TriageResponse = { status: 'ok' };
        fetchMock.post(
          { url: '/json/v3/triage', body: triageRequest },
          { status: 200, body: triageResponse },
        );

        const endPromise = eventPromise('end-task');
        await digestDetailsSkPO.triageSkPO.clickButton('negative');
        await endPromise;
      });

      it('shows an error message upon a triage conflict', async () => {
        const triageResponse: TriageResponse = {
          status: 'conflict',
          conflict: {
            grouping: {
              source_type: 'infra',
              name: 'dots-legend-sk_too-many-digests',
            },
            digest: '6246b773851984c726cb2e1cb13510c2',
            expected_label_before: 'untriaged',
            actual_label_before: 'positive',
          },
        };
        fetchMock.post(
          { url: '/json/v3/triage', body: triageRequest },
          { status: 200, body: triageResponse },
        );

        const errorPromise = eventPromise<CustomEvent<ErrorSkEventDetail>>('error-sk');
        await digestDetailsSkPO.triageSkPO.clickButton('negative');
        const detail = await errorPromise;
        expect(detail.detail.message).to.equal(
          'Triage conflict: Attempted to triage from positive to negative, but the digest\'s '
          + 'current label is untriaged. It is possible that another user triaged this digest. '
          + 'Try refreshing the page.',
        );
      });
    });
  });

  describe('layout with changelist id, positive and negative references', () => {
    beforeEach(() => {
      digestDetailsSk.groupings = deepCopy(groupingsResponse);
      digestDetailsSk.details = deepCopy(typicalDetails);
      digestDetailsSk.commits = deepCopy(twoHundredCommits);
      digestDetailsSk.changeListID = '12345';
      digestDetailsSk.crs = 'github';
    });

    it('includes changelist id on the appropriate links', async () => {
      expect(await digestDetailsSkPO.imageCompareSkPO.getImageCaptionHrefs()).to.deep.equal([
        '/detail?test=dots-legend-sk_too-many-digests'
            + '&digest=6246b773851984c726cb2e1cb13510c2&changelist_id=12345&crs=github',
        '/detail?test=dots-legend-sk_too-many-digests&'
            + 'digest=99c58c7002073346ff55f446d47d6311&changelist_id=12345&crs=github',
      ]);

      expect(await digestDetailsSkPO.getDiffPageLink()).to.equal(
        '/diff?test=dots-legend-sk_too-many-digests&left=6246b773851984c726cb2e1cb13510c2'
            + '&right=99c58c7002073346ff55f446d47d6311&changelist_id=12345&crs=github',
      );
    });

    it('passes changeListID and crs to appropriate subelements', async () => {
      expect(await digestDetailsSkPO.dotsLegendSkPO.getDigestHrefs()).to.deep.equal([
        '/detail?test=dots-legend-sk_too-many-digests&'
            + 'digest=6246b773851984c726cb2e1cb13510c2&changelist_id=12345&crs=github',
        '/detail?test=dots-legend-sk_too-many-digests&'
            + 'digest=99c58c7002073346ff55f446d47d6311&changelist_id=12345&crs=github',
        '/detail?test=dots-legend-sk_too-many-digests&'
            + 'digest=ec3b8f27397d99581e06eaa46d6d5837&changelist_id=12345&crs=github',
      ]);
    });

    describe('RPC requests', () => {
      afterEach(() => {
        expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
        fetchMock.reset();
      });

      it('includes changelist id when triaging', async () => {
        const triageRequest: TriageRequestV3 = {
          deltas: [
            {
              grouping: {
                source_type: 'infra',
                name: 'dots-legend-sk_too-many-digests',
              },
              digest: '6246b773851984c726cb2e1cb13510c2',
              label_before: 'positive',
              label_after: 'negative',
            },
          ],
          changelist_id: '12345',
          crs: 'github',
        };
        const triageResponse: TriageResponse = { status: 'ok' };
        fetchMock.post(
          { url: '/json/v3/triage', body: triageRequest },
          { status: 200, body: triageResponse },
        );

        const endPromise = eventPromise('end-task');
        await digestDetailsSkPO.triageSkPO.clickButton('negative');
        await endPromise;
      });
    });
  });
});
