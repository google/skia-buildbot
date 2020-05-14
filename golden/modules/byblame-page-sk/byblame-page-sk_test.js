import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  canvaskit, gm, svg, trstatus,
} from './demo_data';
import {
  setUpElementUnderTest,
  eventPromise,
  expectQueryStringToEqual,
} from '../test_util';
import { testOnlySetSettings } from '../settings';

describe('byblame-page-sk', () => {
  const newInstance = setUpElementUnderTest('byblame-page-sk');

  const newByblamePageSk = (opts = {}) => {
    testOnlySetSettings({
      defaultCorpus: opts.defaultCorpus || 'gm',
      baseRepoURL: opts.baseRepoUrl || 'https://skia.googlesource.com/skia.git',
    });
    return newInstance();
  };

  beforeEach(async () => {
    // Clear query string before each test case. This is needed for test cases
    // that exercise the stateReflector and the browser's back/forward buttons.
    setQueryString('');
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  it('shows loading indicator', async () => {
    fetchMock.get('/json/trstatus', trstatus);

    // We'll resolve this RPC later to give the "loading" text a chance to show.
    let resolveByBlameRpc;
    fetchMock.get(
      '/json/byblame?query=source_type%3Dgm',
      new Promise((resolve) => resolveByBlameRpc = resolve),
    );

    // Instantiate page, but don't wait for it to load as we want to see the
    // "loading" text.
    const event = eventPromise('end-task');
    const byblamePageSk = newByblamePageSk();

    // Make these assertions immediately, i.e. do not wait for the page to load.
    expect($$('.entries', byblamePageSk).innerText).to.equal('Loading untriaged digests...');
    expectHasEmptyBlames(byblamePageSk);

    // Resolve RPC. This allows the page to finish loading.
    resolveByBlameRpc(gm);

    // Assert that page finished loading.
    await event;
  });

  it('correctly renders a page with empty results', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dcanvaskit', canvaskit);

    const endTask = eventPromise('end-task');
    const byblamePageSk = newByblamePageSk({ defaultCorpus: 'canvaskit' });
    await endTask;

    expectQueryStringToEqual('');
    expectCorporaToBe(byblamePageSk, ['canvaskit', 'gm (114)', 'svg (18)']);
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expect($$('.entries', byblamePageSk).innerText)
      .to.equal('No untriaged digests.');
    expectHasEmptyBlames(byblamePageSk);
  });

  it('renders blames for default corpus if URL does not include a corpus',
    async () => {
      fetchMock.get('/json/trstatus', trstatus);
      fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);

      const endTask = eventPromise('end-task');
      const byblamePageSk = newByblamePageSk({ defaultCorpus: 'gm' });
      await endTask;

      expectQueryStringToEqual(''); // No state reflected to the URL.
      expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
      expectHasGmBlames(byblamePageSk);
    });

  it('renders blames for corpus specified in URL', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);
    setQueryString('?corpus=svg');

    const endTask = eventPromise('end-task');
    const byblamePageSk = newByblamePageSk({ defaultCorpus: 'gm' });
    await endTask;

    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);
  });

  it('switches corpora when corpus-selector-sk is clicked', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);

    const endTask = eventPromise('end-task');
    const byblamePageSk = newByblamePageSk({ defaultCorpus: 'gm' });
    await endTask;

    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
    expectHasGmBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'svg (18)');

    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);
  });

  it('responds to back and forward browser buttons', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dcanvaskit', canvaskit);
    fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);

    // Populate window.history by setting the query string to a random value.
    // We'll then test that we can navigate back to this state by using the
    // browser's back button.
    setQueryString('?hello=world');

    // Clear the query string before loading the component. This will be the
    // state at component instantiation. We'll test that the user doesn't get
    // stuck at the state at component creation when pressing the back button.
    setQueryString('');

    const endTask = eventPromise('end-task');
    const byblamePageSk = newByblamePageSk({ defaultCorpus: 'gm' });
    await endTask;

    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
    expectHasGmBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'svg (18)');
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'canvaskit');
    expectQueryStringToEqual('?corpus=canvaskit');
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expectHasCanvaskitBlames(byblamePageSk);

    await goBack();
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);

    // State at component instantiation.
    await goBack();
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
    expectHasGmBlames(byblamePageSk);

    // State before the component was instantiated.
    await goBack();
    expectQueryStringToEqual('?hello=world');

    await goForward();
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
    expectHasGmBlames(byblamePageSk);

    await goForward();
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);

    await goForward();
    expectQueryStringToEqual('?corpus=canvaskit');
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expectHasCanvaskitBlames(byblamePageSk);
  });

  describe('Base repository URL', () => {
    beforeEach(() => {
      fetchMock.get('/json/trstatus', trstatus);
      fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    });

    it('renders commit links correctly with repo hosted on googlesource',
      async () => {
        const endTask = eventPromise('end-task');
        const byblamePageSk = newByblamePageSk({
          defaultCorpus: 'gm',
          baseRepoUrl: 'https://skia.googlesource.com/skia.git',
        });
        await endTask;

        expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
        expectHasGmBlames(byblamePageSk);
        expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://skia.googlesource.com/skia.git/+show/05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
        );
      });

    it('renders commit links correctly with repo hosted on GitHub',
      async () => {
        const endTask = eventPromise('end-task');
        const byblamePageSk = await newByblamePageSk({
          defaultCorpus: 'gm',
          baseRepoUrl: 'https://github.com/google/skia',
        });
        await endTask;

        expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
        expectHasGmBlames(byblamePageSk);
        expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://github.com/google/skia/commit/05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
        );
      });
  });

  describe('RPC failures', () => {
    it('handles /json/trstatus RPC failure', async () => {
      fetchMock.get('/json/trstatus', 500);
      fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);

      // The corpus-selector-sk will fetch /json/trstatus, fail and emit a
      // fetch-error event.
      const fetchError = eventPromise('fetch-error');
      const endTask = eventPromise('end-task'); // But blames will load.
      const byblamePageSk = await newByblamePageSk();
      await endTask;
      await fetchError;

      // Empty corpus selector due to RPC error.
      expect($('corpus-selector-sk li')).to.be.empty;

      // But the page has a default corpus and thus loads successfully.
      expectHasGmBlames(byblamePageSk);
    });

    it('handles /json/byblame RPC failure', async () => {
      fetchMock.get('/json/trstatus', trstatus);
      fetchMock.get('glob:/json/byblame*', 500);

      const fetchError = eventPromise('fetch-error');
      newByblamePageSk();
      await fetchError;

      expectHasEmptyBlames();
    });
  });
});

function setQueryString(string) {
  history.pushState(
    null,
    '',
    window.location.origin + window.location.pathname + string,
  );
}

function selectCorpus(byblamePageSk, corpus) {
  const event = eventPromise('end-task');
  $$(`corpus-selector-sk li[title="${corpus}"]`, byblamePageSk).click();
  return event;
}

function goBack() {
  const event = eventPromise('end-task');
  history.back();
  return event;
}

function goForward() {
  const event = eventPromise('end-task');
  history.forward();
  return event;
}

function expectCorporaToBe(byblamePageSk, corpora) {
  expect($('corpus-selector-sk li').map((li) => li.innerText))
    .to.deep.equal(corpora);
}

function expectSelectedCorpusToBe(byblamePageSk, corpus) {
  expect($$('corpus-selector-sk li.selected', byblamePageSk).innerText)
    .to.equal(corpus);
}

function expectHasEmptyBlames(byblamePageSk) {
  expectBlames(byblamePageSk, 0);
}

function expectHasCanvaskitBlames(byblamePageSk) {
  expectHasEmptyBlames(byblamePageSk);
}

function expectHasGmBlames(byblamePageSk) {
  // Triage links for first and last entries obtained from the demo page.
  expectBlames(
    byblamePageSk,
    6,
    '/search?blame=4edb719f1bc49bae585ff270df17f08039a96b6c:252cdb782418949651cc5eb7d467c57ddff3d1c7:a1050ed2b1120613d9ae9587e3c0f4116e17337f:3f7c865936cc808af26d88bc1f5740a29cfce200:05f6a01bf9fd25be9e5fff4af5505c3945058b1d&unt=true&head=true&query=source_type%3Dgm',
    '/search?blame=342fbc54844d0d3fc9d20e20b45115db1e33395b&unt=true&head=true&query=source_type%3Dgm',
  );
}

function expectHasSvgBlames(byblamePageSk) {
  // Triage links for first and last entries obtained from the demo page.
  expectBlames(
    byblamePageSk,
    5,
    '/search?blame=d2c67f44f8c2351e60e6ee224a060e916cd44f34&unt=true&head=true&query=source_type%3Dsvg',
    '/search?blame=e1e197186238d8d304a39db9f94258d9584a8973&unt=true&head=true&query=source_type%3Dsvg',
  );
}

function expectBlames(
  byblamePageSk,
  numBlames,
  firstTriageLinkHref,
  lastTriageLinkHref,
) {
  const entries = $('byblameentry-sk', byblamePageSk);
  expect(entries).to.have.length(numBlames);

  // Spot check first and last entries.
  if (firstTriageLinkHref) {
    expect($$('a.triage', entries[0]).href)
      .to.have.string(firstTriageLinkHref);
  }
  if (lastTriageLinkHref) {
    expect($$('a.triage', entries[entries.length - 1]).href)
      .to.have.string(lastTriageLinkHref);
  }
}

function expectFirstCommitLinkHrefToBe(byblamePageSk, expectedHref) {
  const firstCommitLinkSelector = 'byblameentry-sk:first-child ul.blames a:first-child';
  const actualHref = $$(firstCommitLinkSelector, byblamePageSk).href;
  expect(actualHref).to.equal(expectedHref);
}
