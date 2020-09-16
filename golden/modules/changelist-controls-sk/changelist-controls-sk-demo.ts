import './index';

import { ChangelistControlsSk } from './changelist-controls-sk';
import { $$ } from 'common-sk/modules/dom';
import { twoPatchSets } from './test_data';

const ele = $$<ChangelistControlsSk>('changelist-controls-sk');
ele!.setSummary(twoPatchSets);
