import './icon-sk.css';
import { IconSk } from './base';

window.customElements.define('menu-icon-sk', class extends IconSk {
  static get _path() { return "M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z"; }
});