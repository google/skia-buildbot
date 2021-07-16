import './index';

import { ChangelistControlsSk } from './changelist-controls-sk';
import { $$ } from 'common-sk/modules/dom';
import { twoPatchsets } from './test_data';

const ele = $$<ChangelistControlsSk>('changelist-controls-sk');
ele!.summary = twoPatchsets;
