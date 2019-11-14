import './index.js'
import { $, $$ } from 'common-sk/modules/dom'
import { deepCopy } from 'common-sk/modules/object'
import { fetchMock } from 'fetch-mock';
import { trstatus } from './test_data';

describe('corpus-selector-sk', () => {
  // Component under test.
  let corpusSelectorSk;

  // Creates a new corpus-selector-sk instance with the given options and
  // attaches it to the DOM. Variable corpusSelectorSk is set to the new
  // instance.
  function newCorpusSelectorSk(options) {
    corpusSelectorSk = document.createElement('corpus-selector-sk');
    if (options && options.attributes) {
      for (let attribute in options.attributes) {
        corpusSelectorSk.setAttribute(attribute, options.attributes[attribute]);
      }
    }
    document.body.appendChild(corpusSelectorSk);
    if (options && options.corpusRendererFn) {
      corpusSelectorSk.corpusRendererFn = options.corpusRendererFn;
    }
  }

  // Same as newCorpusSelectorSk, except it returns a promise that resolves when
  // the corpora is loaded.
  function loadCorpusSelectorSk(options) {
    const loaded = event('corpus-selector-sk-loaded');
    newCorpusSelectorSk(options);
    return loaded;
  }

  const corpora =
      () => $("li", corpusSelectorSk).map((li) => li.innerText);

  const selectedCorpusLiText = () => {
    const li = $$('li.selected', corpusSelectorSk);
    return li ? li.innerText : null;
  };

  let clock;

  beforeEach(() => {
    clock = sinon.useFakeTimers();
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (corpusSelectorSk) {
      document.body.removeChild(corpusSelectorSk);
      corpusSelectorSk = null;
    }

    fetchMock.reset();
    clock.restore();
  });

  it('shows loading indicator', () => {
    fetchMock.get('/json/trstatus', trstatus);
    newCorpusSelectorSk(); // We don't wait for the corpora to load.
    expect(corpusSelectorSk.innerText).to.equal('Loading corpora...');
  });

  it('renders corpora with unspecified default corpus', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk();
    expect(corpora()).to.deep.equal(
        ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg']);
    expect(corpusSelectorSk.selectedCorpus).to.be.null;
    expect(selectedCorpusLiText()).to.be.null;
  });

  it('renders corpora with default corpus', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({attributes: {'default-corpus': 'gm'}});
    expect(corpora()).to.deep.equal(
        ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg']);
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText()).to.equal('gm');
  });

  it('renders corpora with custom function', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    const options = {
      corpusRendererFn:
          (c) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`,
    };
    await loadCorpusSelectorSk(options);
    expect(corpora()).to.deep.equal([
        'canvaskit : 2 / 2',
        'colorImage : 0 / 1',
        'gm : 61 / 1494',
        'image : 22 / 35',
        'pathkit : 0 / 0',
        'skp : 0 / 1',
        'svg : 19 / 21']);
  });

  it('selects corpus and triggers corpus change event on click', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({attributes: {'default-corpus': 'gm'}});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText()).to.equal('gm');

    // Click on 'svg' corpus.
    const corpusSelected = event('corpus-selected');
    $$('li[title="svg"]', corpusSelectorSk).click();
    const ev = await corpusSelected;

    // Assert that selected corpus changed.
    expect(ev.detail.corpus).to.equal('svg');
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText()).to.equal('svg');
  });

  it('can set the selected corpus programmatically', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({attributes: {'default-corpus': 'gm'}});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText()).to.equal('gm');

    // Select corpus 'svg' programmatically.
    const corpusSelected = event('corpus-selected');
    corpusSelectorSk.selectedCorpus = 'svg';
    const ev = await corpusSelected;

    // Assert that selected corpus changed.
    expect(ev.detail.corpus).to.equal('svg');
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText()).to.equal('svg');
  });

  it('does not trigger corpus change event if selected corpus is clicked',
      async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({attributes: {'default-corpus': 'gm'}});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText()).to.equal('gm');

    // Click on 'gm' corpus.
    corpusSelectorSk.dispatchEvent = sinon.fake();
    $$('li[title="gm"]', corpusSelectorSk).click();

    // Assert that selected corpus didn't change and that no event was emitted.
    expect(corpusSelectorSk.dispatchEvent.callCount).to.equal(0);
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText()).to.equal('gm');
  });

  it('updates automatically with the specified frequency', async () => {
    // Mock /json/trstatus such that the negativeCounts will increase by 1000
    // after each call.
    let updatedStatus = deepCopy(trstatus);
    const fakeRpcEndpoint = sinon.fake(() => {
      const retval = deepCopy(updatedStatus);
      updatedStatus.corpStatus.forEach((corp) => corp.negativeCount += 1000);
      return retval;
    });
    fetchMock.get('/json/trstatus', fakeRpcEndpoint);

    // Initial load.
    await loadCorpusSelectorSk({
      corpusRendererFn:
          (c) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`,
      attributes: {
        'update-freq-seconds': '10'
      }
    });
    expect(fakeRpcEndpoint.callCount).to.equal(1);
    expect(corpora()).to.deep.equal([
      'canvaskit : 2 / 2',
      'colorImage : 0 / 1',
      'gm : 61 / 1494',
      'image : 22 / 35',
      'pathkit : 0 / 0',
      'skp : 0 / 1',
      'svg : 19 / 21']);

    // First update.
    let updated = event('corpus-selector-sk-loaded', 0);
    clock.tick(10000);
    expect(fakeRpcEndpoint.callCount).to.equal(2);
    await updated;
    expect(corpora()).to.deep.equal([
      'canvaskit : 2 / 1002',
      'colorImage : 0 / 1001',
      'gm : 61 / 2494',
      'image : 22 / 1035',
      'pathkit : 0 / 1000',
      'skp : 0 / 1001',
      'svg : 19 / 1021']);

    // Second update.
    updated = event('corpus-selector-sk-loaded', 0);
    clock.tick(10000);
    expect(fakeRpcEndpoint.callCount).to.equal(3);
    await updated;
    expect(corpora()).to.deep.equal([
      'canvaskit : 2 / 2002',
      'colorImage : 0 / 2001',
      'gm : 61 / 3494',
      'image : 22 / 2035',
      'pathkit : 0 / 2000',
      'skp : 0 / 2001',
      'svg : 19 / 2021']);
  });

  it('does not update if update frequency is not specified', async () => {
    const fakeRpcEndpoint = sinon.fake.returns(trstatus);
    fetchMock.get('/json/trstatus', fakeRpcEndpoint);

    await loadCorpusSelectorSk();
    expect(fakeRpcEndpoint.callCount).to.equal(1);

    clock.tick(Number.MAX_SAFE_INTEGER);
    expect(fakeRpcEndpoint.callCount).to.equal(1);
  });

  it('should emit event "fetch-error" on RPC failure', async () => {
    fetchMock.get('/json/trstatus', 500);

    const fetchError = event('fetch-error');
    newCorpusSelectorSk();
    await fetchError;

    expect(corpora()).to.be.empty;
  })
});

// TODO(lovisolo): Move to test_util.js.
// Returns a promise that will resolve when the given event is caught at the
// document's body element, or reject if the event isn't caught within the given
// amount of time.
//
// Set timeoutMillis = 0 to skip call to setTimeout(). This is necessary on
// tests that simulate the passing of time with sinon.useFakeTimers(), which
// could trigger the timeout before the promise has a chance to catch the event.
//
// Sample usage:
//
//   it('should trigger a custom event', async () => {
//     const myEvent = event('quack');
//     // Emits new CustomEvent('quack', {detail: {foo: 'bar'}, bubbles: true});
//     doSomethingThatTriggersMyEvent();
//     const ev = await myEvent;
//     expect(ev.detail.foo).to.equal('bar');
//   });
function event(event, timeoutMillis = 5000) {
  // The executor function passed as a constructor argument to the Promise
  // object is executed immediately. This guarantees that the event handler
  // is added to document.body before returning.
  return new Promise((resolve, reject) => {
    let timeout;
    const handler = (e) => {
      document.body.removeEventListener(event, handler);
      clearTimeout(timeout);
      resolve(e);
    };
    // Skip setTimeout() call with timeoutMillis = 0. Useful when faking time in
    // tests with sinon.useFakeTimers(). See
    // https://sinonjs.org/releases/v7.5.0/fake-timers/.
    if (timeoutMillis !== 0) {
      timeout = setTimeout(() => {
        document.body.removeEventListener(event, handler);
        reject(new Error(`timed out after ${timeoutMillis} ms ` +
            `while waiting to catch event "${event}"`));
      }, timeoutMillis);
    }
    document.body.addEventListener(event, handler);
  });
}
