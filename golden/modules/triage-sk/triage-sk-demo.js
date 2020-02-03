import './index.js';
import { $$ } from 'common-sk/modules/dom';
import { isPuppeteerTest } from '../demo_util';

const log = (message) => {
  const log = $$("#event-log");
  const entry = `${new Date().toISOString()}\t${message}\n`;
  log.value = entry + log.value;
};

const triageSk = document.createElement('triage-sk');
triageSk.addEventListener('change', (e) => log(e.detail));

const clearSelection = $$('#clear-selection');
clearSelection.addEventListener('click', () => {
  triageSk.value = '';
  log('empty');
});

const container = $$('#container');
container.insertBefore(triageSk, clearSelection);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  $$('#event-log-container').style.display = 'none';
}
