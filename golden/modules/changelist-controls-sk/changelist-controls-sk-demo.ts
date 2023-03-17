import './index';

import { $$ } from '../../../infra-sk/modules/dom';
import { ChangelistControlsSk } from './changelist-controls-sk';
import { twoPatchsets } from './test_data';

const ele = $$<ChangelistControlsSk>('changelist-controls-sk');
ele!.summary = twoPatchsets;
