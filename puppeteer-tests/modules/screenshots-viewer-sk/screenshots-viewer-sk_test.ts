import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { ScreenshotsViewerSk } from './screenshots-viewer-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { rpcResponse } from './demo_data';

describe('screenshots-viewer-sk', () => {
  const newInstance = setUpElementUnderTest<ScreenshotsViewerSk>('screenshots-viewer-sk');

  let screenshotsViewerSk: ScreenshotsViewerSk;

  beforeEach(async () => {
    fetchMock.getOnce('/rpc/get-screenshots', rpcResponse);
    const loadedEvent = eventPromise('loaded');
    screenshotsViewerSk = newInstance();
    await loadedEvent;
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // Expect that the RPC was called.
    fetchMock.reset();
  });

  const getScreenshotsFromUI = (): Map<string, string[]> => {
    const ui = new Map<string, string[]>();

    Array.from(screenshotsViewerSk.querySelectorAll<HTMLElement>('.application'))
      .forEach((app) => {
        const appName = app.querySelector<HTMLElement>('.application-name')!.innerText;
        const testNames = Array.from(app.querySelectorAll<HTMLElement>('.test-name'))
          .map((testName) => testName.innerText);
        ui.set(appName, testNames);
      });

    return ui;
  };

  it('shows all tests grouped by application', () => {
    const ui = getScreenshotsFromUI();
    expect(Array.from(ui.keys())).to.deep.equal(['my-app', 'another-app']);
    expect(ui.get('my-app')).to.deep.equal(['alpha', 'beta', 'gamma']);
    expect(ui.get('another-app')).to.deep.equal(['delta', 'epsilon']);
  });

  it('filters results and clears the filter', () => {
    const clearBtn = screenshotsViewerSk.querySelector<HTMLButtonElement>('.filter button')!;

    enterFilter('l');
    let ui = getScreenshotsFromUI();
    expect(Array.from(ui.keys())).to.deep.equal(['my-app', 'another-app']);
    expect(ui.get('my-app')).to.deep.equal(['alpha']);
    expect(ui.get('another-app')).to.deep.equal(['delta', 'epsilon']);

    enterFilter('myapp'); // The lack of a dash (myapp vs. my-app) tests the fuzzy filter.
    ui = getScreenshotsFromUI();
    expect(Array.from(ui.keys())).to.deep.equal(['my-app']);
    expect(ui.get('my-app')).to.deep.equal(['alpha', 'beta', 'gamma']);

    clearBtn.click();
    ui = getScreenshotsFromUI();
    expect(Array.from(ui.keys())).to.deep.equal(['my-app', 'another-app']);
    expect(ui.get('my-app')).to.deep.equal(['alpha', 'beta', 'gamma']);
    expect(ui.get('another-app')).to.deep.equal(['delta', 'epsilon']);
  });

  it('shows no results message when filter maches no results', () => {
    enterFilter('this filter matches no results');
    const ui = getScreenshotsFromUI();
    expect(ui.size).to.equal(0);
    expect(screenshotsViewerSk.querySelector<HTMLElement>('.no-results')!.innerText)
      .to.equal('No screenshots match "this filter matches no results".');
  });

  const enterFilter = (filter: string) => {
    const filterInput = screenshotsViewerSk.querySelector<HTMLInputElement>('.filter input')!;
    filterInput.value = filter;
    filterInput.dispatchEvent(new Event('input', { bubbles: true }));
  };
});
