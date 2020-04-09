import './index';

import { $$ } from 'common-sk/modules/dom';
import { twoPatchSets } from './test_data';

const ele = $$('changelist-controls-sk');
ele.setSummary(twoPatchSets);
