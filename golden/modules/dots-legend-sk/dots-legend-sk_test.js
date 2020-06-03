import './index';
import { $ } from 'common-sk/modules/dom';
import {
  DOT_STROKE_COLORS,
  DOT_FILL_COLORS,
  MAX_UNIQUE_DIGESTS,
} from '../dots-sk/constants';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('dots-legend-sk', () => {
  const newInstance = setUpElementUnderTest('dots-legend-sk');

  let dotsLegendSk;
  beforeEach(() => dotsLegendSk = newInstance());

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

    it('renders dots correctly', () => {
      expect(dotColors(dotsLegendSk)).to.deep.equal([
        [DOT_STROKE_COLORS[0], DOT_FILL_COLORS[0]],
        [DOT_STROKE_COLORS[1], DOT_FILL_COLORS[1]],
        [DOT_STROKE_COLORS[2], DOT_FILL_COLORS[2]],
        [DOT_STROKE_COLORS[3], DOT_FILL_COLORS[3]],
        [DOT_STROKE_COLORS[4], DOT_FILL_COLORS[4]],
      ]);
    });

    it('renders digests correctly', () => {
      expect(digests(dotsLegendSk)).to.deep.equal([
        '00000000000000000000000000000000',
        '11111111111111111111111111111111',
        '22222222222222222222222222222222',
        '33333333333333333333333333333333',
        '44444444444444444444444444444444',
      ]);
    });

    it('renders digest links correctly', () => {
      const digestLinkFor = (d) => `/detail?test=My%20Test&digest=${d}`;
      expect(digestLinks(dotsLegendSk)).to.deep.equal([
        digestLinkFor('00000000000000000000000000000000'),
        digestLinkFor('11111111111111111111111111111111'),
        digestLinkFor('22222222222222222222222222222222'),
        digestLinkFor('33333333333333333333333333333333'),
        digestLinkFor('44444444444444444444444444444444'),
      ]);
    });

    it('renders status icons correctly', () => {
      expect(statusIcons(dotsLegendSk)).to.deep.equal([
        'untriaged',
        'positive',
        'negative',
        'negative',
        'positive',
      ]);
    });

    it('renders diff links correctly', () => {
      const diffLinkFor = (d) => '/diff?test=My%20Test&left=00000000000000000000000000000000'
          + `&right=${d}`;
      expect(diffLinks(dotsLegendSk)).to.deep.equal([
        diffLinkFor('11111111111111111111111111111111'),
        diffLinkFor('22222222222222222222222222222222'),
        diffLinkFor('33333333333333333333333333333333'),
        diffLinkFor('44444444444444444444444444444444'),
      ]);
    });

    describe('with issue number', () => {
      beforeEach(() => {
        dotsLegendSk.test = 'My Test';
        dotsLegendSk.issue = '123456';
      });

      it('renders digest links correctly', () => {
        const digestLinkFor = (d) => `/detail?test=My%20Test&digest=${d}&issue=123456`;
        expect(digestLinks(dotsLegendSk)).to.deep.equal([
          digestLinkFor('00000000000000000000000000000000'),
          digestLinkFor('11111111111111111111111111111111'),
          digestLinkFor('22222222222222222222222222222222'),
          digestLinkFor('33333333333333333333333333333333'),
          digestLinkFor('44444444444444444444444444444444'),
        ]);
      });

      it('renders diff links correctly', () => {
        const diffLinkFor = (d) => '/diff?test=My%20Test&left=00000000000000000000000000000000'
            + `&right=${d}&issue=123456`;
        expect(diffLinks(dotsLegendSk)).to.deep.equal([
          diffLinkFor('11111111111111111111111111111111'),
          diffLinkFor('22222222222222222222222222222222'),
          diffLinkFor('33333333333333333333333333333333'),
          diffLinkFor('44444444444444444444444444444444'),
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

    it('renders dots correctly', () => {
      expect(dotColors(dotsLegendSk)).to.deep.equal([
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

    it('renders digests correctly', () => {
      expect(digests(dotsLegendSk)).to.deep.equal([
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

    it('renders status icons correctly', () => {
      expect(statusIcons(dotsLegendSk)).to.deep.equal([
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

    it('renders diff links correctly', () => {
      const diffLinkFor = (d) => '/diff?test=My%20Test&left=00000000000000000000000000000000'
        + `&right=${d}`;
      expect(diffLinks(dotsLegendSk)).to.deep.equal([
        diffLinkFor('11111111111111111111111111111111'),
        diffLinkFor('22222222222222222222222222222222'),
        diffLinkFor('33333333333333333333333333333333'),
        diffLinkFor('44444444444444444444444444444444'),
        diffLinkFor('55555555555555555555555555555555'),
        diffLinkFor('66666666666666666666666666666666'),
        diffLinkFor('77777777777777777777777777777777'),
        diffLinkFor('88888888888888888888888888888888'),
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

    it('renders dots correctly', () => {
      expect(dotColors(dotsLegendSk)).to.deep.equal([
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

    it('renders digests correctly', () => {
      expect(digests(dotsLegendSk)).to.deep.equal([
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

    it('renders status icons correctly', () => {
      expect(statusIcons(dotsLegendSk)).to.deep.equal([
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

    it('renders diff links correctly', () => {
      const diffLinkFor = (d) => '/diff?test=My%20Test&left=00000000000000000000000000000000'
          + `&right=${d}`;
      expect(diffLinks(dotsLegendSk)).to.deep.equal([
        diffLinkFor('11111111111111111111111111111111'),
        diffLinkFor('22222222222222222222222222222222'),
        diffLinkFor('33333333333333333333333333333333'),
        diffLinkFor('44444444444444444444444444444444'),
        diffLinkFor('55555555555555555555555555555555'),
        diffLinkFor('66666666666666666666666666666666'),
        diffLinkFor('77777777777777777777777777777777'),
      ]);
    });
  });
});

// Takes a color represented as an RGB string (e.g. "rgb(10, 187, 204)") and
// returns the equivalent hex string (e.g. "#0ABBCC").
const rgbToHex = (rgb) => `#${rgb.match(/rgb\((\d+), (\d+), (\d+)\)/)
  .slice(1) // ['10', '187', '204'].
  .map((x) => parseInt(x)) // [10, 187, 204]
  .map((x) => x.toString(16)) // ['a', 'bb', 'cc']
  .map((x) => x.padStart(2, '0')) // ['0a', 'bb', 'cc']
  .map((x) => x.toUpperCase()) // ['0A', 'BB', 'CC']
  .join('')}`; // '0ABBCC'

// Returns the dot colors as an array of arrays of the form
// ["stroke color", "fill color"], where the colors are represented as hex
// strings (e.g. "#AABBCC").
const dotColors = (dotsLegendSk) => $('div.dot', dotsLegendSk)
  .map((dot) => [
    rgbToHex(dot.style.borderColor),
    rgbToHex(dot.style.backgroundColor),
  ]);

const digests = (dotsLegendSk) => $('a.digest, span.one-of-many-other-digests', dotsLegendSk)
  .map((a) => a.innerText.trim());

// Returns the status icons  as an array of strings. Possible values are
// are "negative", "positive", "untriaged".
const statusIcons = (dotsLegendSk) => $([
  'cancel-icon-sk.negative-icon',
  'check-circle-icon-sk.positive-icon',
  'help-icon-sk.untriaged-icon',
].join(', '), dotsLegendSk)
  .map((icon) => icon.className.replace('-icon', ''));

// Takes an URL string (e.g. "http://example.com/search?q=hello") and returns
// only the path and query string (e.g. "/search?q=hello").
const urlToPathAndQueryString = (urlStr) => {
  const url = new URL(urlStr);
  return url.pathname + url.search;
};

const digestLinks = (dotsLegendSk) => $('a.digest', dotsLegendSk).map((a) => urlToPathAndQueryString(a.href));

const diffLinks = (dotsLegendSk) => $('a.diff', dotsLegendSk).map((a) => urlToPathAndQueryString(a.href));
