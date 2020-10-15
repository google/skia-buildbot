import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { NavigationSk } from './navigation-sk';

const el = document.createElement('navigation-sk') as NavigationSk;
document.querySelector('#container')!.appendChild(el);

el.addEventListener('repo-changed', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
