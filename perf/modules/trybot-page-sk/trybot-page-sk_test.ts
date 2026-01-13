import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { TrybotPageSk } from './trybot-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { progress, ReadOnlyParamSet, TryBotResponse } from '../json';

describe('trybot-page-sk', () => {
  const newInstance = setUpElementUnderTest<TrybotPageSk>('trybot-page-sk');

  let element: TrybotPageSk;
  beforeEach(async () => {
    // Mock the window.perf object
    (window as any).perf = {
      key_order: ['config'],
      radius: 10,
      interesting: 50,
    };

    // Mock the initpage fetch
    fetchMock.get('/_/initpage/?tz=UTC', {
      dataframe: {
        paramset: {
          config: ['8888', '565'],
          arch: ['x86', 'arm'],
        },
        header: [],
        traceset: {},
      },
    });

    element = newInstance();
    // Wait for the initial fetch to complete
    await fetchMock.flush(true);
  });

  afterEach(() => {
    fetchMock.restore();
  });

  it('renders', () => {
    assert.isNotNull(element);
    assert.isNotNull(element.querySelector('tabs-sk'));
  });

  it('initializes query with paramset from fetch', () => {
    const query = element.querySelector('query-sk') as any;
    assert.deepEqual(query.paramset, {
      config: ['8888', '565'],
      arch: ['x86', 'arm'],
    });
  });

  it('switches tabs', async () => {
    const tabs = element.querySelector('tabs-sk') as any;
    // Default is 'commit' (index 0)
    assert.equal(tabs.selected, 0);

    // Switch to 'TryBot' (index 1)
    tabs.selected = 1;
    tabs.dispatchEvent(new CustomEvent('tab-selected-sk', { detail: { index: 1 } }));

    // Verify state update (via private property access for testing)
    assert.equal((element as any).state.kind, 'trybot');
  });

  it('updates state on commit selection', () => {
    const picker = element.querySelector('commit-detail-picker-sk')!;
    picker.dispatchEvent(
      new CustomEvent('commit-selected', {
        detail: {
          commit: {
            offset: 123,
            hash: 'abc',
            ts: 1000,
            author: 'me',
            message: 'test',
          },
        },
      })
    );
    assert.equal((element as any).state.commit_number, 123);
  });

  it('updates state on query change', () => {
    const query = element.querySelector('query-sk')!;
    query.dispatchEvent(
      new CustomEvent('query-change', {
        detail: { q: 'config=8888' },
      })
    );
    assert.equal((element as any).state.query, 'config=8888');
  });

  it('runs trybot analysis', async () => {
    // Setup state via events
    const picker = element.querySelector('commit-detail-picker-sk')!;
    picker.dispatchEvent(
      new CustomEvent('commit-selected', {
        detail: {
          commit: {
            offset: 123,
            hash: 'abc',
            ts: 1000,
            author: 'me',
            message: 'test',
          },
        },
      })
    );

    const query = element.querySelector('query-sk')!;
    query.dispatchEvent(
      new CustomEvent('query-change', {
        detail: { q: 'config=8888' },
      })
    );

    // Mock the run request
    const response: progress.SerializedProgress = {
      status: 'Finished',
      messages: [],
      url: '',
      results: {
        header: [],
        results: [],
        paramset: ReadOnlyParamSet({}),
      } as TryBotResponse,
    };

    fetchMock.post('/_/trybot/load/', response);

    // Wait for render after state updates from events
    // Events trigger _render() inside the component (via stateHasChanged), but lit-html might be async.
    // However, ElementSk _render is usually synchronous unless it uses lit's requestUpdate.
    // Let's assume it's synchronous or we might need to wait.

    const runBtn = element.querySelector('#run') as HTMLElement;
    assert.isNotNull(runBtn);
    runBtn.click();

    await fetchMock.flush(true);

    // Check that results are populated
    assert.isNotNull((element as any).results);
  });
});
