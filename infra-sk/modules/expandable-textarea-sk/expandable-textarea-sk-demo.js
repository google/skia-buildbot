import './index';
import '../theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

const si = document.createElement('expandable-textarea-sk');
si.closed = true;
si.placeholderText = 'Your text here';
si.displayText = 'Toggle this textbox';
$$('#container').appendChild(si);
