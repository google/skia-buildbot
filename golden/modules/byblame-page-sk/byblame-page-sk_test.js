import './index.js'

import { $, $$ } from 'common-sk/modules/dom'
import { canvaskit, gm, svg, fakeGitlogRpc, trstatus } from './demo_data'
import { eventPromise, expectNoUnmatchedCalls } from '../test_util'
import { fetchMock } from 'fetch-mock';

describe('byblame-page-sk', () => {
  // Component under test.
  let byblamePageSk;

  // Instantiate component under test with the given options. Save a reference
  // to variable byblamePageSk.
  function newByblamePageSk(
      {
        defaultCorpus = 'gm',
        baseRepoUrl = 'https://skia.googlesource.com/skia.git',
      } = {}) {
    byblamePageSk = document.createElement('byblame-page-sk');
    byblamePageSk.setAttribute('default-corpus', defaultCorpus);
    byblamePageSk.setAttribute('base-repo-url', baseRepoUrl);
    document.body.appendChild(byblamePageSk);
  }

  // Same as newByblamePageSk, but returns a promise that will resolve when the
  // initial contents finish loading.
  function loadByblamePageSk(options) {
    const endTask = eventPromise('end-task');
    newByblamePageSk(options);
    return endTask;
  }

  function setQueryString(string) {
    history.pushState(
        null,
        '',
        window.location.origin + window.location.pathname + string);
  }

  beforeEach(async () => {
    // Clear query string before each test case. This is needed for test cases
    // that exercise the stateReflector and the browser's back/forward buttons.
    setQueryString('');
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (byblamePageSk) {
      document.body.removeChild(byblamePageSk);
      byblamePageSk = null;
    }
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    expectNoUnmatchedCalls(fetchMock);
    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  it('shows loading indicator', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);

    // We'll resolve this RPC later to give the "loading" text a chance to show.
    let resolveByBlameRpc;
    fetchMock.get(
        '/json/byblame?query=source_type%3Dgm',
        new Promise((resolve) => resolveByBlameRpc = resolve));

    // Instantiate page, but don't wait for it to load as we want to see the
    // "loading" text.
    const event = eventPromise('end-task');
    newByblamePageSk();

    // Make these assertions immediately, i.e. do not wait for the page to load.
    expect($$('.entries', byblamePageSk).innerText)
        .to.equal('Loading untriaged digests...');
    expectHasEmptyBlames(byblamePageSk);

    // Resolve RPC. This allows the page to finish loading.
    resolveByBlameRpc(gm);

    // Assert that page finished loading.
    await event;
  });

  it('correctly renders a page with empty results', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dcanvaskit', canvaskit);
    await loadByblamePageSk({defaultCorpus: 'canvaskit'});
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expect($$('.entries', byblamePageSk).innerText)
        .to.equal('No untriaged digests.');
    expectHasEmptyBlames(byblamePageSk);
  });

  it('renders blames for default corpus if URL does not include a corpus',
      async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);
    await loadByblamePageSk({defaultCorpus: 'gm'});
    expectQueryStringToEqual(''); // No state reflected to the URL.
    expectSelectedCorpusToBe(byblamePageSk, 'gm');
    expectHasGmBlames(byblamePageSk);
  });

  it('renders blames for corpus specified in URL', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);
    fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);
    setQueryString('?corpus=svg');
    await loadByblamePageSk({defaultCorpus: 'gm'});
    expectSelectedCorpusToBe(byblamePageSk, 'svg');
    expectHasSvgBlames(byblamePageSk);
  });

  it('switches corpora when corpus-selector-sk is clicked', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);
    fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);

    await loadByblamePageSk({defaultCorpus: 'gm'});
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm');
    expectHasGmBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'svg');
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg');
    expectHasSvgBlames(byblamePageSk);
  });

  it('responds to back and forward browser buttons', async () => {
    fetchMock.get('/json/trstatus', trstatus);
    fetchMock.get('/json/byblame?query=source_type%3Dcanvaskit', canvaskit);
    fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('/json/byblame?query=source_type%3Dsvg', svg);
    fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);

    // Populate window.history by setting the query string to a random value.
    // We'll then test that we can navigate back to this state by using the
    // browser's back button.
    setQueryString('?hello=world');

    // Clear the query string before loading the component. This will be the
    // state at component instantiation. We'll test that the user doesn't get
    // stuck at the state at component creation when pressing the back button.
    setQueryString('');

    await loadByblamePageSk({defaultCorpus: 'gm'});
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm');
    expectHasGmBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'svg');
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg');
    expectHasSvgBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'canvaskit');
    expectQueryStringToEqual('?corpus=canvaskit');
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expectHasCanvaskitBlames(byblamePageSk);

    await goBack();
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg');
    expectHasSvgBlames(byblamePageSk);

    // State at component instantiation.
    await goBack();
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm');
    expectHasGmBlames(byblamePageSk);

    // State before the component was instantiated.
    await goBack();
    expectQueryStringToEqual('?hello=world');

    await goForward();
    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm');
    expectHasGmBlames(byblamePageSk);

    await goForward();
    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg');
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
      fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);
    });

    it('renders commit links correctly with repo hosted on googlesource',
        async () => {
      await loadByblamePageSk({
        defaultCorpus: 'gm',
        baseRepoUrl: 'https://skia.googlesource.com/skia.git',
      });
      expectSelectedCorpusToBe(byblamePageSk, 'gm');
      expectHasGmBlames(byblamePageSk);
      expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://skia.googlesource.com/skia.git/+/85c3d68f2539ed7a1e71f6c9d12baaf9e6be59d8');
    });

    it('renders commit links correctly with repo hosted on GitHub',
        async () => {
      await loadByblamePageSk({
        defaultCorpus: 'gm',
        baseRepoUrl: 'https://github.com/google/skia',
      });
      expectSelectedCorpusToBe(byblamePageSk, 'gm');
      expectHasGmBlames(byblamePageSk);
      expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://github.com/google/skia/commit/85c3d68f2539ed7a1e71f6c9d12baaf9e6be59d8');
    });

    function expectFirstCommitLinkHrefToBe(byblamePageSk, expectedHref) {
      const firstCommitLinkSelector =
          'byblameentry-sk:first-child ul.blames a:first-child';
      const actualHref = $$(firstCommitLinkSelector, byblamePageSk).href;
      expect(actualHref).to.equal(expectedHref);
    }
  });

  describe('RPC failures', () => {
    it('handles /json/trstatus RPC failure', async () => {
      fetchMock.get('/json/trstatus', 500);
      fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
      fetchMock.get('glob:/json/gitlog*', fakeGitlogRpc);

      // The corpus-selector-sk will fetch /json/trstatus, fail and emit a
      // fetch-error event.
      const fetchError = eventPromise('fetch-error');
      await loadByblamePageSk(); // Wait for blames to load (end-task).
      await fetchError;

      // Empty corpus selector due to RPC error.
      expect($('corpus-selector-sk li')).to.be.empty;

      // But the page has a default corpus and thus loads successfully.
      expectHasGmBlames(byblamePageSk);
    });

    it('handles /json/byblame RPC failure', async () => {
      fetchMock.get('/json/trstatus', trstatus);
      fetchMock.get('glob:/json/byblame*', 500);

      const error = eventPromise('fetch-error');
      newByblamePageSk();
      await error;

      expectHasEmptyBlames();
    });

    it('handles /json/gitlog RPC failure', async () => {
      fetchMock.get('/json/trstatus', trstatus);
      fetchMock.get('/json/byblame?query=source_type%3Dgm', gm);
      fetchMock.get('glob:/json/gitlog*', 500);

      const error = eventPromise('fetch-error');
      newByblamePageSk();
      await error;

      expectHasEmptyBlames();
    });
  });

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

  function expectQueryStringToEqual(expected) {
    expect(window.location.search).to.equal(expected);
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
        20,
        '/search?blame=85c3d68f2539ed7a1e71f6c9d12baaf9e6be59d8&unt=true&head=true&query=source_type%3Dgm',
        '/search?blame=f5ad3f421e112108d44da73dc8e3bd8a513748c4&unt=true&head=true&query=source_type%3Dgm');

  }

  function expectHasSvgBlames(byblamePageSk) {
    // Triage links for first and last entries obtained from the demo page.
    expectBlames(
        byblamePageSk,
        5,
        '/search?blame=d2c67f44f8c2351e60e6ee224a060e916cd44f34&unt=true&head=true&query=source_type%3Dsvg',
        '/search?blame=e1e197186238d8d304a39db9f94258d9584a8973&unt=true&head=true&query=source_type%3Dsvg');
  }

  function expectBlames(
      byblamePageSk,
      numBlames,
      firstTriageLinkHref,
      lastTriageLinkHref) {
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
});
