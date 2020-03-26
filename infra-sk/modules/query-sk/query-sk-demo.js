import './index';

const q = document.querySelector('query-sk');
const events = document.querySelector('#events');
q.addEventListener('query-change', (e) => {
  events.textContent = JSON.stringify(e.detail, null, '  ');
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
q.paramset = paramset;
q.key_order = ['test', 'units'];

document.querySelector('#swap').addEventListener('click', () => {
  n = (n + 1) % 2;
  q.paramset = [paramset, paramset2][n];
});
