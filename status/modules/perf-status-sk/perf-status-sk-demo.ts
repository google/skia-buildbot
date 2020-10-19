import './index';
import fetchMock from 'fetch-mock';

fetchMock.getOnce('https://perf.skia.org/_/alerts/', { alerts: 5 });
const el = document.createElement('perf-status-sk');
document.querySelector('#container')?.appendChild(el);
