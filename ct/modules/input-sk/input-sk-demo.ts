import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { InputSk } from './input-sk';

const si = document.createElement('input-sk') as InputSk;
si.label = 'Type something here';
($$('#container') as HTMLElement).appendChild(si);
