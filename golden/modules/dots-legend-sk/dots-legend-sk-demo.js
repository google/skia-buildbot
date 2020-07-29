import './index';
import { $$ } from 'common-sk/modules/dom';

const someDigests = [
  { digest: 'ce0a9d2b546b25e00e39a33860cb72b6', status: 'untriaged' },
  { digest: '34e87ca0f753cf4c884fa01af6c08be9', status: 'positive' },
  { digest: '8ee9a2c61e9f12e6243f07423302f26a', status: 'negative' },
  { digest: '6174fc17b06e6ff9e383da3f6952ad68', status: 'negative' },
  { digest: 'dcccd6998b47f60ab28dcff17ae57ed2', status: 'positive' },
];

const tooManyDigests = [
  ...someDigests,
  { digest: '92d9faf80a25750629118018716387df', status: 'positive' },
  { digest: '1bc4771dcee95d97b2758a1e1945cc40', status: 'untriaged' },
  { digest: 'fdefcfdfee6fc5f64a128345d10a8010', status: 'untriaged' },
  { digest: 'eb84e709671d9d207d2ba20b1da66ce0', status: 'untriaged' },
  { digest: 'b00cb97f0d4dd7b22fb9af5378918d9f', status: 'untriaged' },
];

function newDotsLegendSk(parentSelector, id, digests, clID, test) {
  const dotsLegendSk = document.createElement('dots-legend-sk');
  dotsLegendSk.id = id;
  dotsLegendSk.digests = digests;
  dotsLegendSk.changeListID = clID;
  dotsLegendSk.test = test;
  dotsLegendSk.totalDigests = digests.length;
  $$(parentSelector).appendChild(dotsLegendSk);
}

newDotsLegendSk(
  '#some-digests-container',
  'some-digests',
  someDigests,
  '123456',
  'My-Test',
);

newDotsLegendSk(
  '#too-many-digests-container',
  'too-many-digests',
  tooManyDigests,
  '123456',
  'My-Test',
);
