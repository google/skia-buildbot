import './index';
import {
  DOT_STROKE_COLORS,
  DOT_FILL_COLORS,
  MAX_UNIQUE_DIGESTS,
} from '../dots-sk/constants';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DotsLegendSk } from './dots-legend-sk';
import { DotsLegendSkPO } from './dots-legend-sk_po';
import { expect } from 'chai';

describe('dots-legend-sk', () => {
  const newInstance = setUpElementUnderTest<DotsLegendSk>('dots-legend-sk');

  let dotsLegendSk: DotsLegendSk;
  let dotsLegendSkPO: DotsLegendSkPO;

  beforeEach(() => {
    dotsLegendSk = newInstance();
    dotsLegendSkPO = new DotsLegendSkPO(dotsLegendSk);
  });

  describe('with less than MAX_UNIQUE_DIGESTS unique digests', () => {
    beforeEach(() => {
      dotsLegendSk.test = 'My Test';
      dotsLegendSk.digests = [
        { digest: '00000000000000000000000000000000', status: 'untriaged' },
        { digest: '11111111111111111111111111111111', status: 'positive' },
        { digest: '22222222222222222222222222222222', status: 'negative' },
        { digest: '33333333333333333333333333333333', status: 'negative' },
        { digest: '44444444444444444444444444444444', status: 'positive' },
      ];
      // We set this to 5 to mimic what the server would give us - it is important that this
      // match or exceed the length of digests, so as to draw properly.
      dotsLegendSk.totalDigests = 5;
      expect(dotsLegendSk.digests.length).to.equal(dotsLegendSk.totalDigests);
    });

    it('renders dots correctly', async () => {
      expect(await dotsLegendSkPO.getDotBorderAndBackgroundColors()).to.deep.equal([
        [DOT_STROKE_COLORS[0], DOT_FILL_COLORS[0]],
        [DOT_STROKE_COLORS[1], DOT_FILL_COLORS[1]],
        [DOT_STROKE_COLORS[2], DOT_FILL_COLORS[2]],
        [DOT_STROKE_COLORS[3], DOT_FILL_COLORS[3]],
        [DOT_STROKE_COLORS[4], DOT_FILL_COLORS[4]],
      ]);
    });

    it('renders digests correctly', async () => {
      expect(await dotsLegendSkPO.getDigests()).to.deep.equal([
        '00000000000000000000000000000000',
        '11111111111111111111111111111111',
        '22222222222222222222222222222222',
        '33333333333333333333333333333333',
        '44444444444444444444444444444444',
      ]);
    });

    it('renders digest links correctly', async () => {
      const digestHrefFor = (d: string) => `/detail?test=My Test&digest=${d}`;
      expect(await dotsLegendSkPO.getDigestHrefs()).to.deep.equal([
        digestHrefFor('00000000000000000000000000000000'),
        digestHrefFor('11111111111111111111111111111111'),
        digestHrefFor('22222222222222222222222222222222'),
        digestHrefFor('33333333333333333333333333333333'),
        digestHrefFor('44444444444444444444444444444444'),
      ]);
    });

    it('renders status icons correctly', async () => {
      expect(await dotsLegendSkPO.getTriageIconLabels()).to.deep.equal([
        'untriaged',
        'positive',
        'negative',
        'negative',
        'positive',
      ]);
    });

    it('renders diff links correctly', async () => {
      const diffHrefFor =
          (d: string) => `/diff?test=My Test&left=00000000000000000000000000000000&right=${d}`;
      expect(await dotsLegendSkPO.getDiffHrefs()).to.deep.equal([
        diffHrefFor('11111111111111111111111111111111'),
        diffHrefFor('22222222222222222222222222222222'),
        diffHrefFor('33333333333333333333333333333333'),
        diffHrefFor('44444444444444444444444444444444'),
      ]);
    });

    describe('with CL ID and crs', () => {
      beforeEach(() => {
        dotsLegendSk.test = 'My Test';
        dotsLegendSk.changeListID = '123456';
        dotsLegendSk.crs = 'gerrit';
      });

      it('renders digest links correctly', async () => {
        const digestHrefFor = (d:string) =>
            `/detail?test=My Test&digest=${d}&changelist_id=123456&crs=gerrit`;
        expect(await dotsLegendSkPO.getDigestHrefs()).to.deep.equal([
          digestHrefFor('00000000000000000000000000000000'),
          digestHrefFor('11111111111111111111111111111111'),
          digestHrefFor('22222222222222222222222222222222'),
          digestHrefFor('33333333333333333333333333333333'),
          digestHrefFor('44444444444444444444444444444444'),
        ]);
      });

      it('renders diff links correctly', async () => {
        const diffHrefFor = (d: string) =>
            '/diff?test=My Test&left=00000000000000000000000000000000' +
            `&right=${d}&changelist_id=123456&crs=gerrit`;
        expect(await dotsLegendSkPO.getDiffHrefs()).to.deep.equal([
          diffHrefFor('11111111111111111111111111111111'),
          diffHrefFor('22222222222222222222222222222222'),
          diffHrefFor('33333333333333333333333333333333'),
          diffHrefFor('44444444444444444444444444444444'),
        ]);
      });
    });
  });

  describe('with exactly MAX_UNIQUE_DIGESTS unique digests', () => {
    beforeEach(() => {
      dotsLegendSk.test = 'My Test';
      dotsLegendSk.digests = [
        { digest: '00000000000000000000000000000000', status: 'untriaged' },
        { digest: '11111111111111111111111111111111', status: 'positive' },
        { digest: '22222222222222222222222222222222', status: 'negative' },
        { digest: '33333333333333333333333333333333', status: 'negative' },
        { digest: '44444444444444444444444444444444', status: 'positive' },
        { digest: '55555555555555555555555555555555', status: 'positive' },
        { digest: '66666666666666666666666666666666', status: 'untriaged' },
        { digest: '77777777777777777777777777777777', status: 'untriaged' },
        { digest: '88888888888888888888888888888888', status: 'untriaged' },
      ];
      expect(dotsLegendSk.digests.length).to.equal(MAX_UNIQUE_DIGESTS);

      dotsLegendSk.totalDigests = MAX_UNIQUE_DIGESTS;
    });

    it('renders dots correctly', async () => {
      expect(await dotsLegendSkPO.getDotBorderAndBackgroundColors()).to.deep.equal([
        [DOT_STROKE_COLORS[0], DOT_FILL_COLORS[0]],
        [DOT_STROKE_COLORS[1], DOT_FILL_COLORS[1]],
        [DOT_STROKE_COLORS[2], DOT_FILL_COLORS[2]],
        [DOT_STROKE_COLORS[3], DOT_FILL_COLORS[3]],
        [DOT_STROKE_COLORS[4], DOT_FILL_COLORS[4]],
        [DOT_STROKE_COLORS[5], DOT_FILL_COLORS[5]],
        [DOT_STROKE_COLORS[6], DOT_FILL_COLORS[6]],
        [DOT_STROKE_COLORS[7], DOT_FILL_COLORS[7]],
        [DOT_STROKE_COLORS[8], DOT_FILL_COLORS[8]],
      ]);
    });

    it('renders digests correctly', async () => {
      expect(await dotsLegendSkPO.getDigests()).to.deep.equal([
        '00000000000000000000000000000000',
        '11111111111111111111111111111111',
        '22222222222222222222222222222222',
        '33333333333333333333333333333333',
        '44444444444444444444444444444444',
        '55555555555555555555555555555555',
        '66666666666666666666666666666666',
        '77777777777777777777777777777777',
        '88888888888888888888888888888888',
      ]);
    });

    it('renders status icons correctly', async () => {
      expect(await dotsLegendSkPO.getTriageIconLabels()).to.deep.equal([
        'untriaged',
        'positive',
        'negative',
        'negative',
        'positive',
        'positive',
        'untriaged',
        'untriaged',
        'untriaged',
      ]);
    });

    it('renders diff links correctly', async () => {
      const diffHrefFor = (d: string) =>
          `/diff?test=My Test&left=00000000000000000000000000000000&right=${d}`;
      expect(await dotsLegendSkPO.getDiffHrefs()).to.deep.equal([
        diffHrefFor('11111111111111111111111111111111'),
        diffHrefFor('22222222222222222222222222222222'),
        diffHrefFor('33333333333333333333333333333333'),
        diffHrefFor('44444444444444444444444444444444'),
        diffHrefFor('55555555555555555555555555555555'),
        diffHrefFor('66666666666666666666666666666666'),
        diffHrefFor('77777777777777777777777777777777'),
        diffHrefFor('88888888888888888888888888888888'),
      ]);
    });
  });

  describe('with more than MAX_UNIQUE_DIGESTS unique digests', () => {
    beforeEach(() => {
      dotsLegendSk.test = 'My Test';
      dotsLegendSk.digests = [
        { digest: '00000000000000000000000000000000', status: 'untriaged' },
        { digest: '11111111111111111111111111111111', status: 'positive' },
        { digest: '22222222222222222222222222222222', status: 'negative' },
        { digest: '33333333333333333333333333333333', status: 'negative' },
        { digest: '44444444444444444444444444444444', status: 'positive' },
        { digest: '55555555555555555555555555555555', status: 'positive' },
        { digest: '66666666666666666666666666666666', status: 'untriaged' },
        { digest: '77777777777777777777777777777777', status: 'untriaged' },
        { digest: '88888888888888888888888888888888', status: 'untriaged' },
        // The API currently tops out at 9 unique digests (counting the one digest that is part of
        // the search results. The tenth unique digest below is included to test that this component
        // behaves gracefully in the event that the API behavior changes and the front-end and
        // back-end code fall out of sync.
        { digest: '99999999999999999999999999999999', status: 'untriaged' },
      ];

      dotsLegendSk.totalDigests = 123;
    });

    it('renders dots correctly', async () => {
      expect(await dotsLegendSkPO.getDotBorderAndBackgroundColors()).to.deep.equal([
        [DOT_STROKE_COLORS[0], DOT_FILL_COLORS[0]],
        [DOT_STROKE_COLORS[1], DOT_FILL_COLORS[1]],
        [DOT_STROKE_COLORS[2], DOT_FILL_COLORS[2]],
        [DOT_STROKE_COLORS[3], DOT_FILL_COLORS[3]],
        [DOT_STROKE_COLORS[4], DOT_FILL_COLORS[4]],
        [DOT_STROKE_COLORS[5], DOT_FILL_COLORS[5]],
        [DOT_STROKE_COLORS[6], DOT_FILL_COLORS[6]],
        [DOT_STROKE_COLORS[7], DOT_FILL_COLORS[7]],
        [DOT_STROKE_COLORS[8], DOT_FILL_COLORS[8]],
      ]);
    });

    it('renders digests correctly', async () => {
      expect(await dotsLegendSkPO.getDigests()).to.deep.equal([
        '00000000000000000000000000000000',
        '11111111111111111111111111111111',
        '22222222222222222222222222222222',
        '33333333333333333333333333333333',
        '44444444444444444444444444444444',
        '55555555555555555555555555555555',
        '66666666666666666666666666666666',
        '77777777777777777777777777777777',
        'One of 115 other digests (123 in total).',
      ]);
    });

    it('renders status icons correctly', async () => {
      expect(await dotsLegendSkPO.getTriageIconLabels()).to.deep.equal([
        'untriaged',
        'positive',
        'negative',
        'negative',
        'positive',
        'positive',
        'untriaged',
        'untriaged',
      ]);
    });

    it('renders diff links correctly', async () => {
      const diffHrefFor = (d: string) =>
          `/diff?test=My Test&left=00000000000000000000000000000000&right=${d}`;
      expect(await dotsLegendSkPO.getDiffHrefs()).to.deep.equal([
        diffHrefFor('11111111111111111111111111111111'),
        diffHrefFor('22222222222222222222222222222222'),
        diffHrefFor('33333333333333333333333333333333'),
        diffHrefFor('44444444444444444444444444444444'),
        diffHrefFor('55555555555555555555555555555555'),
        diffHrefFor('66666666666666666666666666666666'),
        diffHrefFor('77777777777777777777777777777777'),
      ]);
    });
  });
});
