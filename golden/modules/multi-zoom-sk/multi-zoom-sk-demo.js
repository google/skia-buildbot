import './index';

import { $$ } from '../../../common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';

setImageEndpointsForDemos();
const ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftDigest: '99c58c7002073346ff55f446d47d6311',
  rightDigest: '6246b773851984c726cb2e1cb13510c2',
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
$$('#normal').appendChild(ele);
