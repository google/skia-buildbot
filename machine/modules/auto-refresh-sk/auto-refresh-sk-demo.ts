import { $$ } from 'common-sk/modules/dom';
import { AutoRefreshSk } from './auto-refresh-sk';
import './index';

const element = $$<AutoRefreshSk>('auto-refresh-sk', document)!;
element.refreshing = false;
element.addEventListener('refresh-page', (e) => {
    document.querySelector('#events')!.textContent = `${JSON.stringify(e, null, '  ')}\n${Date.now()}`;
});
