import './index.js'
import { $$ } from 'common-sk/modules/dom'
import { traces, commits } from './demo_data'

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
