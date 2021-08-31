import './index';

import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import {
  canvaskit, gm, svg, trstatus,
} from './demo_data';
import {
  setUpElementUnderTest,
  eventPromise,
  setQueryString,
  expectQueryStringToEqual,
} from '../../../infra-sk/modules/test_util';
import { testOnlySetSettings } from '../settings';
import { ByBlamePageSk } from './byblame-page-sk';

describe('byblame-page-sk', () => {
  const newInstance = setUpElementUnderTest<ByBlamePageSk>('byblame-page-sk');

  const loadedByblamePageSk = (opts: {defaultCorpus?: string, baseRepoUrl?: string} = {}): Promise<ByBlamePageSk> => {
    testOnlySetSettings({
      defaultCorpus: opts.defaultCorpus || 'gm',
      baseRepoURL: opts.baseRepoUrl || 'https://skia.googlesource.com/skia.git',
    });
    return new Promise((resolve) => {
      let endTaskCalls = 0;
      const byBlamePageSk = newInstance((ele) => {
        ele.addEventListener('end-task', () => {
          endTaskCalls++;
          if (endTaskCalls === 2) { // Wait for 2 RPCs to finish.
            resolve(byBlamePageSk);
          }
        });
      });
    });
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
    fetchMock.get('/json/v2/trstatus', trstatus);

    // We'll resolve this RPC later to give the "loading" text a chance to show.
    let resolveByBlameRpc = (_: {}) => {};
    fetchMock.get(
      '/json/v2/byblame?query=source_type%3Dgm',
      new Promise((resolve) => resolveByBlameRpc = resolve),
    );

    // Instantiate page, but don't wait for it to load as we want to see the
    // "loading" text.
    const event = eventPromise('end-task');
    testOnlySetSettings({ defaultCorpus: 'gm' });
    const byblamePageSk = newInstance();

    // Make these assertions immediately, i.e. do not wait for the page to load.
    expect($$<HTMLElement>('.entries', byblamePageSk)!.innerText)
      .to.equal('Loading untriaged digests...');
    expectHasEmptyBlames(byblamePageSk);

    // Resolve RPC. This allows the page to finish loading.
    resolveByBlameRpc(gm);

    // Assert that page finished loading.
    await event;
  });

  it('correctly renders a page with empty results', async () => {
    fetchMock.get('/json/v2/trstatus', trstatus);
    fetchMock.get('/json/v2/byblame?query=source_type%3Dcanvaskit', canvaskit);

    const byblamePageSk = await loadedByblamePageSk({ defaultCorpus: 'canvaskit' });

    expectQueryStringToEqual('');
    expectCorporaToBe(byblamePageSk, ['canvaskit', 'gm (114)', 'svg (18)']);
    expectSelectedCorpusToBe(byblamePageSk, 'canvaskit');
    expect($$<HTMLElement>('.entries', byblamePageSk)!.innerText)
      .to.equal('No untriaged digests for corpus canvaskit.');
    expectHasEmptyBlames(byblamePageSk);
  });

  it('renders blames for default corpus if URL does not include a corpus',
    async () => {
      fetchMock.get('/json/v2/trstatus', trstatus);
      fetchMock.get('/json/v2/byblame?query=source_type%3Dgm', gm);

      const byblamePageSk = await loadedByblamePageSk({ defaultCorpus: 'gm' });

      expectQueryStringToEqual(''); // No state reflected to the URL.
      expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
      expectHasGmBlames(byblamePageSk);
    });

  it('renders blames for corpus specified in URL', async () => {
    fetchMock.get('/json/v2/trstatus', trstatus);
    fetchMock.get('/json/v2/byblame?query=source_type%3Dsvg', svg);
    setQueryString('?corpus=svg');

    const byblamePageSk = await loadedByblamePageSk({ defaultCorpus: 'gm' });

    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);
  });

  it('switches corpora when corpus-selector-sk is clicked', async () => {
    fetchMock.get('/json/v2/trstatus', trstatus);
    fetchMock.get('/json/v2/byblame?query=source_type%3Dgm', gm);
    fetchMock.get('/json/v2/byblame?query=source_type%3Dsvg', svg);

    const byblamePageSk = await loadedByblamePageSk({ defaultCorpus: 'gm' });

    expectQueryStringToEqual('');
    expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
    expectHasGmBlames(byblamePageSk);

    await selectCorpus(byblamePageSk, 'svg (18)');

    expectQueryStringToEqual('?corpus=svg');
    expectSelectedCorpusToBe(byblamePageSk, 'svg (18)');
    expectHasSvgBlames(byblamePageSk);
  });

  describe('Base repository URL', () => {
    beforeEach(() => {
      fetchMock.get('/json/v2/trstatus', trstatus);
      fetchMock.get('/json/v2/byblame?query=source_type%3Dgm', gm);
    });

    it('renders commit links correctly with repo hosted on googlesource',
      async () => {
        const byblamePageSk = await loadedByblamePageSk({
          defaultCorpus: 'gm',
          baseRepoUrl: 'https://skia.googlesource.com/skia.git',
        });

        expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
        expectHasGmBlames(byblamePageSk);
        expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://skia.googlesource.com/skia.git/+show/05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
        );
      });

    it('renders commit links correctly with repo hosted on GitHub',
      async () => {
        const byblamePageSk = await loadedByblamePageSk({
          defaultCorpus: 'gm',
          baseRepoUrl: 'https://github.com/google/skia',
        });

        expectSelectedCorpusToBe(byblamePageSk, 'gm (114)');
        expectHasGmBlames(byblamePageSk);
        expectFirstCommitLinkHrefToBe(
          byblamePageSk,
          'https://github.com/google/skia/commit/05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
        );
      });
  });
});

function selectCorpus(byblamePageSk: ByBlamePageSk, corpus: string) {
  const event = eventPromise('end-task');
  $$<HTMLElement>(`corpus-selector-sk li[title="${corpus}"]`, byblamePageSk)!.click();
  return event;
}

function expectCorporaToBe(byblamePageSk: ByBlamePageSk, corpora: string[]) {
  expect($<HTMLLIElement>('corpus-selector-sk li').map((li) => li.innerText))
    .to.deep.equal(corpora);
}

function expectSelectedCorpusToBe(byblamePageSk: ByBlamePageSk, corpus: string) {
  expect($$<HTMLLIElement>('corpus-selector-sk li.selected', byblamePageSk)!.innerText)
    .to.equal(corpus);
}

function expectHasEmptyBlames(byblamePageSk: ByBlamePageSk) {
  expectBlames(byblamePageSk, 0);
}

function expectHasGmBlames(byblamePageSk: ByBlamePageSk) {
  // Triage links for first and last entries obtained from the demo page.
  expectBlames(
    byblamePageSk,
    6,
    '/search?blame='
      + '4edb719f1bc49bae585ff270df17f08039a96b6c:252cdb782418949651cc5eb7d467c57ddff3d1c7:'
      + 'a1050ed2b1120613d9ae9587e3c0f4116e17337f:3f7c865936cc808af26d88bc1f5740a29cfce200:'
      + '05f6a01bf9fd25be9e5fff4af5505c3945058b1d&corpus=gm',
    '/search?blame=342fbc54844d0d3fc9d20e20b45115db1e33395b&corpus=gm',
  );
}

function expectHasSvgBlames(byblamePageSk: ByBlamePageSk) {
  // Triage links for first and last entries obtained from the demo page.
  expectBlames(
    byblamePageSk,
    5,
    '/search?blame=d2c67f44f8c2351e60e6ee224a060e916cd44f34&corpus=svg',
    '/search?blame=e1e197186238d8d304a39db9f94258d9584a8973&corpus=svg',
  );
}

function expectBlames(
  byblamePageSk: ByBlamePageSk,
  numBlames: number,
  firstTriageLinkHref?: string,
  lastTriageLinkHref?: string,
) {
  const entries = $('byblameentry-sk', byblamePageSk);
  expect(entries).to.have.length(numBlames);

  // Spot check first and last entries.
  if (firstTriageLinkHref) {
    expect($$<HTMLAnchorElement>('a.triage', entries[0])!.href)
      .to.have.string(firstTriageLinkHref);
  }
  if (lastTriageLinkHref) {
    expect($$<HTMLAnchorElement>('a.triage', entries[entries.length - 1])!.href)
      .to.have.string(lastTriageLinkHref);
  }
}

function expectFirstCommitLinkHrefToBe(byblamePageSk: ByBlamePageSk, expectedHref: string) {
  const firstCommitLinkSelector = 'byblameentry-sk:first-child ul.blames a:first-child';
  const actualHref = $$<HTMLAnchorElement>(firstCommitLinkSelector, byblamePageSk)!.href;
  expect(actualHref).to.equal(expectedHref);
}
