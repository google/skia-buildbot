import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import { CommitDetailPickerSk } from './commit-detail-picker-sk';
import { Commit } from '../json/all';

Date.now = () => Date.parse('2020-03-22T00:00:00.000Z');

fetchMock.post('/_/cidRange/', (): Commit[] => [
  {
    offset: 43389,
    author: 'Avinash Parchuri (aparchur@google.com)',
    message: 'Reland "[skottie] Add onTextProperty support into ',
    url:
      'https://skia.googlesource.com/skia/+show/3a543aafd4e68af182ef88572086c094cd63f0b2',
    hash: '3a543aafd4e68af182ef88572086c094cd63f0b2',
    ts: 1565099441,
  },
  {
    offset: 43390,
    author: 'Robert Phillips (robertphillips@google.com)',
    message: 'Use GrComputeTightCombinedBufferSize in GrMtlGpu::',
    url:
      'https://skia.googlesource.com/skia/+show/bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    hash: 'bdb0919dcc6a700b41492c53ecf06b40983d13d7',
    ts: 1565107798,
  },
  {
    offset: 43391,
    author: 'Hal Canary (halcanary@google.com)',
    message: 'experimental/editor: interface no longer uses stri',
    url:
      'https://skia.googlesource.com/skia/+show/e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    hash: 'e45bf6a603b7990f418eaf19ef0e2a2e59a9f449',
    ts: 1565220328,
  },
]);

// eslint-disable-next-line import/first
import './index';

const evt = document.querySelector('#evt')!;

document
  .querySelectorAll<CommitDetailPickerSk>('commit-detail-picker-sk')
  .forEach((panel) => {
    panel.addEventListener('commit-selected', (e: Event) => {
      evt.textContent = JSON.stringify((e as CustomEvent).detail, null, '  ');
    });
  });

document.querySelector<CommitDetailPickerSk>('#darkmode-picker button')!.click();
