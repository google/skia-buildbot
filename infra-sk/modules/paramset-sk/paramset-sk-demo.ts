import './index';
import { ParamSet } from 'common-sk/modules/query';
import { ParamSetSk, ParamSetSkClickEventDetail } from './paramset-sk';

const paramSet1: ParamSet = {
  arch: ['Arm7', 'Arm64', 'x86_64', 'x86'],
  bench_type: ['micro', 'playback', 'recording'],
  compiler: ['GCC', 'MSVC', 'Clang'],
  cpu_or_gpu: ['GPU', 'CPU'],
};

const paramSet2: ParamSet = {
  arch: ['Arm7'],
  bench_type: ['playback', 'recording'],
  compiler: [],
  extra_config: ['Android', 'Android_NoGPUThreads'],
  cpu_or_gpu: ['GPU'],
};

const title1 = 'ParamSet 1';
const title2 = 'ParamSet 2';

const allParamSetSks: ParamSetSk[] = [];

const findParamSetSk = (selector: string) => {
  const paramSetSk = document.querySelector<ParamSetSk>(selector)!;
  allParamSetSks.push(paramSetSk);
  return paramSetSk;
};

let paramSetSk = findParamSetSk('#one-paramset-no-titles');
paramSetSk.paramsets = [paramSet1];

paramSetSk = findParamSetSk('#one-paramset-with-titles');
paramSetSk.paramsets = [paramSet1];
paramSetSk.titles = [title1];

paramSetSk = findParamSetSk('#many-paramsets-no-titles');
paramSetSk.paramsets = [paramSet1, paramSet2];

paramSetSk = findParamSetSk('#many-paramsets-with-titles');
paramSetSk.paramsets = [paramSet1, paramSet2];
paramSetSk.titles = [title1, title2];

paramSetSk = findParamSetSk('#many-paramsets-with-titles-values-clickable');
paramSetSk.paramsets = [paramSet1, paramSet2];
paramSetSk.titles = [title1, title2];
paramSetSk.clickable_values = true;

paramSetSk = findParamSetSk('#many-paramsets-with-titles-keys-and-values-clickable');
paramSetSk.paramsets = [paramSet1, paramSet2];
paramSetSk.titles = [title1, title2];
paramSetSk.clickable = true;

paramSetSk = findParamSetSk('#clickable-plus');
paramSetSk.paramsets = [paramSet1];
paramSetSk.titles = [title1];

paramSetSk = findParamSetSk('#clickable-plus-with-clickable-values');
paramSetSk.paramsets = [paramSet1];
paramSetSk.titles = [title1];

allParamSetSks.forEach((paramSetSk) => {
  paramSetSk.addEventListener('paramset-key-click', (e) => {
    const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
    document.querySelector<HTMLPreElement>('#key-click-event')!.textContent = JSON.stringify(detail, null, '  ');
  });

  paramSetSk.addEventListener('paramset-key-value-click', (e) => {
    const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
    document.querySelector<HTMLPreElement>('#key-value-click-event')!.textContent = JSON.stringify(detail, null, '  ');
  });

  paramSetSk.addEventListener('plus-click', (e) => {
    const detail = (e as CustomEvent<ParamSetSkClickEventDetail>).detail;
    document.querySelector<HTMLPreElement>('#plus-click-event')!.textContent = JSON.stringify(detail, null, '  ');
  });
});

document.querySelector('#highlight')!.addEventListener('click', () => {
  allParamSetSks.forEach(
    (paramSetSk) => paramSetSk.highlight = { arch: 'Arm7', cpu_or_gpu: 'GPU' },
  );
});

document.querySelector('#clear')!.addEventListener('click', () => {
  allParamSetSks.forEach((paramSetSk) => paramSetSk.highlight = {});
});
