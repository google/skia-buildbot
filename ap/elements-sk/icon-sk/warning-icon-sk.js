import './icon-sk.css';
import { IconSk } from './base';

window.customElements.define('warning-icon-sk', class extends IconSk {
  static get _path() { return "M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"; }
});