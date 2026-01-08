import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { TrybotPageSk } from './trybot-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

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

    // Check state reflection (mocked or inferred from implementation)
    // Since stateReflector is private, we can observe side effects or just ensure no crash
    // ideally we would check window.location or element state if public.
    // But for now, just ensure the event handling doesn't error.
  });
});
