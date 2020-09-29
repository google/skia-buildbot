import './theme-chooser-sk';
import './theme-chooser-sk-demo.scss';
import { DARKMODE_CLASS } from './theme-chooser-sk';

// Force the element to use the default mode set in the elements attribute.
window.localStorage.removeItem(DARKMODE_CLASS);
