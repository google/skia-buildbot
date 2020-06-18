import './index';
import fetchMock from 'fetch-mock';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { QueryChooserSk } from './query-chooser-sk';

// Wait 1s before returning the content so we can see the spinner in action.
fetchMock.post('/', { count: Math.floor(Math.random() * 2000) });

const all = document.querySelectorAll<QueryChooserSk>('query-chooser-sk');
all.forEach((ele) => {
  ele.addEventListener<any>(
    'query-change',
    (e: CustomEvent<QuerySkQueryChangeEventDetail>) => {
      document.querySelector('#events')!.textContent = JSON.stringify(
        e.detail,
        null,
        '  '
      );
      ele.paramset = paramset;
      ele.key_order = ['test', 'units'];
    }
  );
});

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
