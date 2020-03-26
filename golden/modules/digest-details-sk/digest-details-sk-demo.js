import './index';
import { digestDetails } from './test_data';
import { $$ } from '../../../common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';

setImageEndpointsForDemos();
const ele = document.createElement('digest-details-sk');
ele.details = digestDetails;
$$('#container').appendChild(ele);
