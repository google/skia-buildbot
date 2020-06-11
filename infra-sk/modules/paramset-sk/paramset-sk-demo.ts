import './index';
import { ParamSetSk, ParamSetSkClickEventDetail } from './paramset-sk';
import { ParamSet } from 'common-sk/modules/query';

const paramset: ParamSet = {
  arch: ['Arm7', 'Arm64', 'x86_64', 'x86'],
  bench_type: ['micro', 'playback', 'recording'],
  compiler: ['GCC', 'MSVC', 'Clang'],
  cpu_or_gpu: ['GPU', 'CPU'],
};

const paramset2: ParamSet = {
  arch: ['Arm7'],
  bench_type: ['playback', 'recording'],
  compiler: [],
  extra_config: ['Android', 'Android_NoGPUThreads'],
  cpu_or_gpu: ['GPU'],
};

const set1 = document.querySelector<ParamSetSk>('#set1')!;
const set2 = document.querySelector<ParamSetSk>('#set2')!;
const set3 = document.querySelector<ParamSetSk>('#set3')!;

const key = document.querySelector<HTMLPreElement>('#key')!;
const value = document.querySelector<HTMLPreElement>('#value')!;

set1.paramsets = [paramset];

set2.paramsets = [paramset, paramset2];
set2.titles = ['Set 1', 'Set 2'];

set3.paramsets = [paramset];
set3.titles = ['Clickable Values Only'];

set2.addEventListener('paramset-key-click', (e: Event) => {
  const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
  key.textContent = JSON.stringify(detail, null, '  ');
});

set2.addEventListener('paramset-key-value-click', (e) => {
  const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
  value.textContent = JSON.stringify(detail, null, '  ');
});

set3.addEventListener('paramset-key-value-click', (e) => {
  const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
  value.textContent = JSON.stringify(detail, null, '  ');
});

document.querySelector('#highlight')!.addEventListener('click', () => {
  set1.highlight = { arch: 'Arm7', cpu_or_gpu: 'GPU' };
  set2.highlight = { arch: 'Arm7', cpu_or_gpu: 'GPU' };
  set3.highlight = { arch: 'Arm7', cpu_or_gpu: 'GPU' };
});

document.querySelector('#clear')!.addEventListener('click', () => {
  set1.highlight = {};
  set2.highlight = {};
  set3.highlight = {};
});
