import './index';
import { GraphListSk, GraphItem } from './graph-list-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { expect } from 'chai';

describe('graph-list-sk', () => {
  const newInstance = setUpElementUnderTest<GraphListSk>('graph-list-sk');

  let element: GraphListSk;

  const createMockItems = (count: number) => {
    return Array.from({ length: count }, (_, i) => {
      const id = (i + 1).toString();
      let generationCount = 0;

      return {
        id,
        generationCount: () => generationCount, // Test helper to check cache
        generateGraph: () => {
          generationCount++;
          const explore = new ExploreSimpleSk(false);
          explore.updateChartHeight = () => {};
          explore.setAttribute('data-test-id', id);

          setTimeout(() => {
            explore.dispatchEvent(new CustomEvent('data-loaded', { bubbles: true }));
          }, 0);
          return explore;
        },
      } as GraphItem & { generationCount: () => number };
    });
  };

  beforeEach(() => {
    (window as any).perf = {
      trace_format: '',
    };
    element = newInstance();
  });

  it('renders initial chunks correctly', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 5;
    element.items = createMockItems(7);

    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    const graphs = element.getGraphs();
    expect(graphs.length).to.equal(5);
  });

  it('loads more graphs when loadMore is called', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 5;
    element.items = createMockItems(7);

    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );
    expect(element.getGraphs().length).to.equal(5);

    element.loadMore();

    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );
    expect(element.getGraphs().length).to.equal(7);
  });

  it('caches graphs and does not regenerate them when removed and re-added', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 5;
    const items = createMockItems(2);
    element.items = items;

    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    // First generation
    expect(items[0].generationCount()).to.equal(1);

    // Remove the item
    await element.removeGraph(items[0].id);
    await element.updateComplete;
    expect(element.getGraphs().length).to.equal(1);

    // Add it back
    await element.addGraph(items[0]);
    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    // It should STILL be 1 because it was pulled from the internal _graphCache
    expect(items[0].generationCount()).to.equal(1);
    expect(element.getGraphs().length).to.equal(2);
  });

  it('preserves the original historical order when items are removed and re-added', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 5;

    // Initial order: [1, 2, 3]
    const items = createMockItems(3);
    element.items = [...items];

    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    // Remove item '2'
    await element.removeGraph('2');
    await element.updateComplete;

    let graphs = element.getGraphs();
    expect(graphs[0].getAttribute('data-test-id')).to.equal('1');
    expect(graphs[1].getAttribute('data-test-id')).to.equal('3');

    // Add item '2' back. It should remember it belongs between 1 and 3,
    // rather than appending it to the very end.
    await element.addGraph(items[1]);
    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    graphs = element.getGraphs();
    expect(graphs.length).to.equal(3);
    expect(graphs[0].getAttribute('data-test-id')).to.equal('1');
    expect(graphs[1].getAttribute('data-test-id')).to.equal('2'); // Order preserved!
    expect(graphs[2].getAttribute('data-test-id')).to.equal('3');
  });

  it('disables pagination and auto-loads new items after Load All is clicked', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 2;
    const items = createMockItems(4);

    // Render first chunk (2 items)
    element.items = items.slice(0, 3);
    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );
    expect(element.getGraphs().length).to.equal(2);

    // Trigger Load All
    element.loadMore(true);
    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );
    expect(element.getGraphs().length).to.equal(3);

    // Because pagination is now permanently disabled, adding a new graph
    // should automatically render it without waiting for a "Load More" click.
    await element.addGraph(items[3]);
    await new Promise((resolve) =>
      element.addEventListener('graphs-loaded', resolve, { once: true })
    );

    expect(element.getGraphs().length).to.equal(4);
  });

  it('queues a Load All command if it is clicked while a chunk is actively loading', async () => {
    element.chunkSize = 2;
    element.visibleLimit = 2;

    // We intentionally make generation slow to simulate network delay
    const slowItems = createMockItems(4).map((item) => {
      item.generateGraph = () => {
        const explore = new ExploreSimpleSk(false);
        explore.updateChartHeight = () => {};
        setTimeout(() => {
          explore.dispatchEvent(new CustomEvent('data-loaded', { bubbles: true }));
        }, 50); // 50ms artificial delay
        return explore;
      };
      return item;
    });

    // Start loading the first 2 items
    element.items = slowItems;

    // Before they finish (while _currentlyLoading is active), hit the Load All queue
    setTimeout(() => {
      // Accessing the private method for testing via any, or you could query the button
      (element as any).triggerLoadAll();
    }, 10);

    // Wait for the full cycle to resolve (first chunk -> queue -> remaining chunks load)
    await new Promise((resolve) => {
      const listener = () => {
        if (element.getGraphs().length === 4) {
          element.removeEventListener('graphs-loaded', listener);
          resolve(null);
        }
      };
      element.addEventListener('graphs-loaded', listener);
    });

    expect(element.getGraphs().length).to.equal(4);
  });
});
