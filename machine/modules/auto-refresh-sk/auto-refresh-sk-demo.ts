import { $$ } from 'common-sk/modules/dom';
import { AutoRefreshSk } from './auto-refresh-sk';
import './index';

const element = $$<AutoRefreshSk>('auto-refresh-sk', document)!;

// Force into a known value at start.
element.refreshing = false;

// Display refresh-page events and when they occur.
element.addEventListener('refresh-page', (e) => {
    document.querySelector('#events')!.textContent = `${JSON.stringify(e, null, '  ')}\n${Date.now()}`;
});
