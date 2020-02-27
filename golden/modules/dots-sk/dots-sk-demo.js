import './index.js';
import { $$ } from 'common-sk/modules/dom';
import { isPuppeteerTest } from '../demo_util';
import { traces, commits } from './demo_data';

const logEventDetail = (e) => {
  const log = $$("#event-log");
  const entry = `Timestamp:    ${new Date().toISOString()}\n`
      + `Event type:   ${e.type}\n`
      + `Event detail: ${JSON.stringify(e.detail)}\n\n`;
  log.value = entry + log.value;
};

const dots = document.createElement('dots-sk');
dots.value = traces;
dots.commits = commits;
dots.addEventListener('show-commits', logEventDetail);
dots.addEventListener('hover', logEventDetail);
$$('#container').appendChild(dots);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  $$('#event-log-container').style.display = 'none';
}