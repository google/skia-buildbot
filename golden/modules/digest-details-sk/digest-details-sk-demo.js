import './index';
import { digestDetails } from './test_data';
import { $$ } from '../../../common-sk/modules/dom';

const ele = document.createElement('digest-details-sk');
ele.details = digestDetails;
$$('#container').appendChild(ele);
