import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

const si = document.createElement('input-sk');
si.label = 'Type something here';
$$('#container').appendChild(si);
