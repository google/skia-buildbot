import './index';

import { QuerySk, QuerySkQueryChangeEventDetail } from './query-sk';

const q = document.querySelectorAll<QuerySk>('query-sk')!;
const events = document.querySelector<HTMLPreElement>('#events')!;
q.forEach((ele) => {
  ele.addEventListener('query-change', (e) => {
    const detail = (e as CustomEvent<QuerySkQueryChangeEventDetail>).detail;
    events.textContent = JSON.stringify(detail, null, '  ');
  });
});

let n = 0;
const paramset = {
  config: ['565', '8888'],
  type: ['CPU', 'GPU'],
  units: ['ms', 'bytes'],
  test: [
    'DeferredSurfaceCopy_discardable',
    'DeferredSurfaceCopy_nonDiscardable',
    'GLInstancedArraysBench_instance',
    'GLInstancedArraysBench_one_0',
    'GLInstancedArraysBench_one_1',
    'GLInstancedArraysBench_one_2',
    'GLInstancedArraysBench_one_4',
    'GLInstancedArraysBench_one_8',
    'GLInstancedArraysBench_two_0',
    'GLInstancedArraysBench_two_1',
    'GLInstancedArraysBench_two_2',
    'GLInstancedArraysBench_two_4',
    'GLInstancedArraysBench_two_8',
    'GLVec4ScalarBench_scalar_1_stage',
    'GLVec4ScalarBench_scalar_2_stage',
  ],
};
const paramset2 = {
  config: ['565'],
  type: ['CPU', 'GPU'],
  test: [
    'DeferredSurfaceCopy_discardable',
    'DeferredSurfaceCopy_nonDiscardable',
  ],
};
q.forEach((ele) => {
  ele.paramset = paramset;
  ele.key_order = ['test', 'units'];
});

document
  .querySelector<HTMLButtonElement>('#swap')!
  .addEventListener('click', () => {
    n = (n + 1) % 2;
    q.forEach((ele) => {
      ele.paramset = [paramset, paramset2][n];
    });
  });

// Set filtering on one of the demo elements.
const activeFilter = document.querySelector<HTMLInputElement>(
  '#activeFilter #fast'
)!;
activeFilter.value = 'one';
activeFilter.dispatchEvent(new InputEvent('input'));
