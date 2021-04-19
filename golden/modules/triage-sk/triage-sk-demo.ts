import './index';
import { isPuppeteerTest } from '../demo_util';
import {LabelOrEmpty, TriageSk} from './triage-sk';

const log = (message: string) => {
  const log = document.querySelector<HTMLTextAreaElement>('#event-log')!;
  const entry = `${new Date().toISOString()}\t${message}\n`;
  log.value = entry + log.value;
};

const triageSk = new TriageSk();
triageSk.addEventListener('change', (e) => log((e as CustomEvent<LabelOrEmpty>).detail));

const clearSelection = document.querySelector('#clear-selection')!;
clearSelection.addEventListener('click', () => {
  triageSk.value = '';
  log('empty');
});

document.querySelector<HTMLDivElement>('#container')!.insertBefore(triageSk, clearSelection);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  document.querySelector<HTMLDivElement>('#event-log-container')!.style.display = 'none';
}
