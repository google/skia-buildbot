import './index';

import { PlotSummarySk } from './plot-summary-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { generateFullDataFrame } from '../dataframe/test_utils';

document.querySelectorAll('plot-summary-sk').forEach((e) =>
  e.addEventListener('summary_selected', (e) => {
    const plotDetails = e.detail;
    document.querySelector('#events')!.textContent = JSON.stringify(plotDetails);
  })
);

// 2023 Oct 1st.
const now = new Date(2023, 9, 1).getTime() / 1000;

// The commits span is more or less arbitrary, the timestamp has different
// steps so in date mode, they will have some curves.
// All the numbers are arbitrary to only produce the different trace curves.
const frames = [
  generateFullDataFrame(
    { begin: 100, end: 120 },
    now,
    2,
    [60 * 60 * 6, 60 * 60],
    [Array.from({ length: 4 }, (_, k) => k)]
  ),
  generateFullDataFrame(
    { begin: 100, end: 120 },
    now,
    2,
    [60 * 60, 60 * 60 * 2],
    [Array.from({ length: 10 }, (_, k) => k)]
  ),
  generateFullDataFrame(
    { begin: 100, end: 120 },
    now,
    2,
    [45 * 60, 45 * 60 * 3],
    [Array.from({ length: 24 }, (_, k) => k)]
  ),
];

window.customElements
  .whenDefined('plot-summary-sk')
  .then(() => {
    const plots = document.querySelectorAll<PlotSummarySk>('plot-summary-sk');
    const readys: Promise<boolean>[] = [];
    plots.forEach((plot) => {
      readys.push(plot.updateComplete);
    });
    return Promise.all(readys);
  })
  .then(() => {
    const plots = document.querySelectorAll<PlotSummarySk>('plot-summary-sk');
    plots.forEach((plot, idx) => {
      plot.dataframe = frames[idx % frames.length];
    });
  });
