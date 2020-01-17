import './index.js';
import { $$ } from 'common-sk/modules/dom';
import { isPuppeteerTest } from '../demo_util';

const logEventDetail = (e) => {
  const log = $$("#event-log");
  const entry = `${new Date().toISOString()}\t${e.detail}\n`
  log.value = entry + log.value;
};

const triageSk = document.createElement('triage-sk');
triageSk.addEventListener('change', logEventDetail);
$$('#container').append(triageSk);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  $$('#event-log-container').style.display = 'none';
}
