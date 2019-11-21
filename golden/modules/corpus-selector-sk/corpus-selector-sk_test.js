import './index.js';
import { $, $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { eventPromise } from '../test_util';
import { fetchMock } from 'fetch-mock';
import { trstatus } from './test_data';

describe('corpus-selector-sk', () => {
  // Component under test.
  let corpusSelectorSk;

  // Creates a new corpus-selector-sk instance with the given options and
  // attaches it to the DOM. Variable corpusSelectorSk is set to the new
  // instance.
  function newCorpusSelectorSk(
      {updateFreqSeconds, corpusRendererFn, selectedCorpus}={}) {
    corpusSelectorSk = document.createElement('corpus-selector-sk');
    if (updateFreqSeconds)
      corpusSelectorSk.setAttribute('update-freq-seconds', updateFreqSeconds);
    if (corpusRendererFn) corpusSelectorSk.corpusRendererFn = corpusRendererFn;
    if (selectedCorpus) corpusSelectorSk.selectedCorpus = selectedCorpus;
    document.body.appendChild(corpusSelectorSk);
  }

  // Same as newCorpusSelectorSk, except it returns a promise that resolves when
  // the corpora is loaded.
  function loadCorpusSelectorSk(options) {
    const loaded = eventPromise('corpus-selector-sk-loaded');
    newCorpusSelectorSk(options);
    return loaded;
  }

  const corporaLiText =
      (el) => $("li", el).map((li) => li.innerText);

  const selectedCorpusLiText = (el) => {
    const li = $$('li.selected', el);
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
    newCorpusSelectorSk(); // Don't wait for the corpora to load.
    expect(corpusSelectorSk.innerText).to.equal('Loading corpora details...');
  });

  it('renders corpora with unspecified default corpus', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk();
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal(
        ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg']);
    expect(corpusSelectorSk.selectedCorpus).to.be.undefined;
    expect(selectedCorpusLiText(corpusSelectorSk)).to.be.null;
  });

  it('renders corpora with default corpus', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({selectedCorpus: 'gm'});
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal(
        ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg']);
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');
  });

  it('renders corpora with custom function', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({
      corpusRendererFn:
          (c) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`
    });
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal([
        'canvaskit : 2 / 2',
        'colorImage : 0 / 1',
        'gm : 61 / 1494',
        'image : 22 / 35',
        'pathkit : 0 / 0',
        'skp : 0 / 1',
        'svg : 19 / 21']);
  });

  it('selects corpus and emits "corpus-selected" event when clicked',
      async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({selectedCorpus: 'gm'});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Click on 'svg' corpus.
    const corpusSelected = eventPromise('corpus-selected');
    $$('li[title="svg"]', corpusSelectorSk).click();
    const ev = await corpusSelected;

    // Assert that selected corpus changed.
    expect(ev.detail.corpus).to.equal('svg');
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('svg');
  });

  it('can set the selected corpus programmatically', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({selectedCorpus: 'gm'});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Select corpus 'svg' programmatically.
    corpusSelectorSk.selectedCorpus = 'svg';

    // Assert that selected corpus changed.
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('svg');
  });

  it('does not trigger corpus change event if selected corpus is clicked',
      async () => {
    fetchMock.get('/json/trstatus', trstatus);
    await loadCorpusSelectorSk({selectedCorpus: 'gm'});
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Click on 'gm' corpus.
    corpusSelectorSk.dispatchEvent = sinon.fake();
    $$('li[title="gm"]', corpusSelectorSk).click();

    // Assert that selected corpus didn't change and that no event was emitted.
    expect(corpusSelectorSk.dispatchEvent.callCount).to.equal(0);
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');
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
      updateFreqSeconds: 10,
    });
    expect(fakeRpcEndpoint.callCount).to.equal(1);
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal([
      'canvaskit : 2 / 2',
      'colorImage : 0 / 1',
      'gm : 61 / 1494',
      'image : 22 / 35',
      'pathkit : 0 / 0',
      'skp : 0 / 1',
      'svg : 19 / 21']);

    // First update.
    let updated = eventPromise('corpus-selector-sk-loaded', 0);
    clock.tick(10000);
    expect(fakeRpcEndpoint.callCount).to.equal(2);
    await updated;
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal([
      'canvaskit : 2 / 1002',
      'colorImage : 0 / 1001',
      'gm : 61 / 2494',
      'image : 22 / 1035',
      'pathkit : 0 / 1000',
      'skp : 0 / 1001',
      'svg : 19 / 1021']);

    // Second update.
    updated = eventPromise('corpus-selector-sk-loaded', 0);
    clock.tick(10000);
    expect(fakeRpcEndpoint.callCount).to.equal(3);
    await updated;
    expect(corporaLiText(corpusSelectorSk)).to.deep.equal([
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

    // RPC end-point called once on creation.
    await loadCorpusSelectorSk();
    expect(fakeRpcEndpoint.callCount).to.equal(1);

    // No further RPC calls after waiting a long time.
    clock.tick(Number.MAX_SAFE_INTEGER);
    expect(fakeRpcEndpoint.callCount).to.equal(1);
  });

  it('stops pinging server for updates after detached from DOM', async () => {
    const fakeRpcEndpoint = sinon.fake.returns(trstatus);
    fetchMock.get('/json/trstatus', fakeRpcEndpoint);

    // RPC end-point called once on creation.
    await loadCorpusSelectorSk({updateFreqSeconds: 10});
    expect(fakeRpcEndpoint.callCount).to.equal(1);

    // Does update.
    clock.tick(20000);
    expect(fakeRpcEndpoint.callCount).to.equal(3);

    // Detach component from DOM.
    document.body.removeChild(corpusSelectorSk);

    // No further RPC calls.
    clock.tick(20000);
    expect(fakeRpcEndpoint.callCount).to.equal(3);

    // Reattach component, otherwise afterEach() will try to remove it and fail.
    document.body.appendChild(corpusSelectorSk);
  });


  it('should emit event "fetch-error" on RPC failure', async () => {
    fetchMock.get('/json/trstatus', 500);

    const fetchError = eventPromise('fetch-error');
    newCorpusSelectorSk();
    await fetchError;

    expect(corporaLiText(corpusSelectorSk)).to.be.empty;
  })
});
