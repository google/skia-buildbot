import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import sinon from 'sinon';
import { expect } from 'chai';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { entry } from './test_data';
import { testOnlySetSettings } from '../settings';
import { ByBlameEntrySk } from './byblameentry-sk';
import { ByBlameEntry } from '../rpc_types';

describe('byblameentry-sk', () => {
  const newInstance = setUpElementUnderTest<ByBlameEntrySk>('byblameentry-sk');

  const newByBlameEntrySk = (byBlameEntry: ByBlameEntry, opts: {baseRepoUrl?: string, corpus?: string} = {}) => {
    testOnlySetSettings({
      baseRepoURL: opts.baseRepoUrl || 'https://skia.googlesource.com/skia.git',
    });
    return newInstance((el) => {
      el.byBlameEntry = byBlameEntry;
      el.corpus = opts.corpus || 'gm';
    });
  };

  let clock: sinon.SinonFakeTimers;

  beforeEach(() => {
    // This is necessary to make commit ages deterministic, and is set to 50
    // seconds after Elisa's commit in the 'full example' test case below.
    clock = sinon.useFakeTimers(1573149864000); // Nov 7, 2019 6:04:24 PM GMT
  });

  afterEach(() => {
    clock.restore();
  });

  describe('full example', () => {
    it('renders correctly', async () => {
      // This is a comprehensive example of a blame group with multiple
      // untriaged digests that could have originated from two different
      // commits, and includes a list of affected tests along with links to an
      // example untriaged digest for each affected test.

      const byBlameEntrySk = newByBlameEntrySk(entry);
      expectTriageLinkEquals(
        byBlameEntrySk,
        '112 untriaged digests',
        // This server-generated blame ID is a colon-separated list of the
        // commit hashes blamed for these untriaged digests.
        '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&corpus=gm',
      );
      expectBlamesListEquals(
        byBlameEntrySk,
        [{
          linkText: 'bbbbbbb',
          linkHref: 'https://skia.googlesource.com/skia.git/+show/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
          commitMessage: 'One glyph() to rule them all!!!',
          author: 'Elisa (elisa@example.com)',
          age: '6h',
        }, {
          linkText: 'aaaaaaa',
          linkHref: 'https://skia.googlesource.com/skia.git/+show/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
          commitMessage: 'flesh out blendmodes through Screen',
          author: 'Joe (joe@example.com)',
          age: '5m',
        }],
      );
      expectNumTestsAffectedEquals(byBlameEntrySk, '7 tests affected.');
      expectAffectedTestsTableEquals(
        byBlameEntrySk,
        [{
          test: 'aarectmodes',
          corpus: 'infra',
          numDigests: 50,
          exampleLinkText: 'c6476baec94eb6a5071606575318e4df',
          exampleLinkHref: '/detail?grouping=name%3Daarectmodes%26source_type%3Dinfra'
            + '&digest=c6476baec94eb6a5071606575318e4df',
        }, {
          test: 'aaxfermodes',
          corpus: 'infra',
          numDigests: 32,
          exampleLinkText: '4acfd6b3a3943cc5d75cd22e966ae6f1',
          exampleLinkHref: '/detail?grouping=name%3Daaxfermodes%26source_type%3Dinfra'
            + '&digest=4acfd6b3a3943cc5d75cd22e966ae6f1',
        }, {
          test: 'hairmodes',
          corpus: 'infra',
          numDigests: 21,
          exampleLinkText: 'f9e20c63b5ce3b58d9b6a90fa3e7224c',
          exampleLinkHref: '/detail?grouping=name%3Dhairmodes%26source_type%3Dinfra'
            + '&digest=f9e20c63b5ce3b58d9b6a90fa3e7224c',
        }, {
          test: 'imagefilters_xfermodes',
          corpus: 'infra',
          numDigests: 5,
          exampleLinkText: '47644613317040264fea6fa815af32e8',
          exampleLinkHref: '/detail?grouping=name%3Dimagefilters_xfermodes%26source_type%3Dinfra'
            + '&digest=47644613317040264fea6fa815af32e8',
        }, {
          test: 'lattice2',
          corpus: 'infra',
          numDigests: 2,
          exampleLinkText: '16e41798ecd59b1523322a57b49cc17f',
          exampleLinkHref: '/detail?grouping=name%3Dlattice2%26source_type%3Dinfra'
            + '&digest=16e41798ecd59b1523322a57b49cc17f',
        }, {
          test: 'xfermodes',
          corpus: 'infra',
          numDigests: 1,
          exampleLinkText: '8fbee03f794c455c4e4842ec2736b744',
          exampleLinkHref: '/detail?grouping=name%3Dxfermodes%26source_type%3Dinfra'
            + '&digest=8fbee03f794c455c4e4842ec2736b744',
        }, {
          test: 'xfermodes3',
          corpus: 'infra',
          numDigests: 1,
          exampleLinkText: 'fed2ff29abe371fc0ec1b2c65dfb3949',
          exampleLinkHref: '/detail?grouping=name%3Dxfermodes3%26source_type%3Dinfra'
            + '&digest=fed2ff29abe371fc0ec1b2c65dfb3949',
        }],
      );
    });
  });

  describe('triage link', () => {
    it('renders link text in singular if there is just 1 digest', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.nDigests = 1;
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectTriageLinkEquals(
        byBlameEntrySk,
        '1 untriaged digest',
        '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&corpus=gm',
      );
    });

    it('includes the right corpus in the link href', async () => {
      const byBlameEntrySk = newByBlameEntrySk(entry, { corpus: 'svg' });
      expectTriageLinkEquals(
        byBlameEntrySk,
        '112 untriaged digests',
        '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&corpus=svg',
      );
    });
  });

  describe('blamelist', () => {
    it('shows "No blamelist" message if there are 0 blames', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.commits = [];
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectBlamesListEquals(byBlameEntrySk, []);
    });

    it('points commit links to GitHub if repo is hosted there', async () => {
      const byBlameEntrySk = newByBlameEntrySk(
        entry,
        { baseRepoUrl: 'https://github.com/google/skia' },
      );
      expectBlamesListEquals(
        byBlameEntrySk,
        [{
          linkText: 'bbbbbbb',
          linkHref: 'https://github.com/google/skia/commit/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
          commitMessage: 'One glyph() to rule them all!!!',
          author: 'Elisa (elisa@example.com)',
          age: '6h',
        }, {
          linkText: 'aaaaaaa',
          linkHref: 'https://github.com/google/skia/commit/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
          commitMessage: 'flesh out blendmodes through Screen',
          author: 'Joe (joe@example.com)',
          age: '5m',
        }],
      );
    });

    it('shows empty commit messages if Git log is empty/missing', async () => {
      const byBlameEntrySk = newByBlameEntrySk(entry);
      expectBlamesListEquals(
        byBlameEntrySk,
        [{
          linkText: 'bbbbbbb',
          linkHref: 'https://skia.googlesource.com/skia.git/+show/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
          commitMessage: '',
          author: 'Elisa (elisa@example.com)',
          age: '6h',
        }, {
          linkText: 'aaaaaaa',
          linkHref: 'https://skia.googlesource.com/skia.git/+show/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
          commitMessage: '',
          author: 'Joe (joe@example.com)',
          age: '5m',
        }],
      );
    });
  });

  describe('affected tests', () => {
    it('renders correctly with nTests = 0', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.nTests = 0;
      testByBlameEntry.affectedTests = [];
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals(byBlameEntrySk, '0 tests affected.');
      expectAffectedTestsTableEquals(byBlameEntrySk, []);
    });

    it('renders correctly with nTests = 1 and 0 affected tests', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.nTests = 1;
      testByBlameEntry.affectedTests = [];
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals(byBlameEntrySk, '1 test affected.');
      expectAffectedTestsTableEquals(byBlameEntrySk, []);
    });

    it('renders correctly with nTests = 2 and 0 affected tests', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.nTests = 2;
      testByBlameEntry.affectedTests = [];
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals(byBlameEntrySk, '2 tests affected.');
      expectAffectedTestsTableEquals(byBlameEntrySk, []);
    });

    it('renders correctly with nTests = 2 and one affected test', async () => {
      const testByBlameEntry = deepCopy(entry);
      testByBlameEntry.nTests = 2;
      testByBlameEntry.affectedTests = [{
        grouping: {
          source_type: 'infra',
          name: 'aarectmodes',
        },
        num: 5,
        sample_digest: 'c6476baec94eb6a5071606575318e4df',
      }];
      const byBlameEntrySk = newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals(byBlameEntrySk, '2 tests affected.');
      expectAffectedTestsTableEquals(
        byBlameEntrySk,
        [{
          test: 'aarectmodes',
          corpus: 'infra',
          numDigests: 5,
          exampleLinkText: 'c6476baec94eb6a5071606575318e4df',
          exampleLinkHref: '/detail?grouping=name%3Daarectmodes%26source_type%3Dinfra'
            + '&digest=c6476baec94eb6a5071606575318e4df',
        }],
      );
    });
  });
});

const expectTriageLinkEquals = (byBlameEntrySk: ByBlameEntrySk, text: string, href: string) => {
  const triageLink = $$<HTMLAnchorElement>('a.triage', byBlameEntrySk)!;
  expect(triageLink.innerText).to.contain(text);
  expect(triageLink.href).to.have.string(href);
};

interface ExpectedBlame {
  linkText: string;
  linkHref: string;
  commitMessage: string;
  author: string;
  age: string;
}

const expectBlamesListEquals = (byBlameEntrySk: ByBlameEntrySk, expectedBlames: ExpectedBlame[]) => {
  const noBlameList = $$<HTMLParagraphElement>('p.no-blamelist', byBlameEntrySk)!;
  const blames = $<HTMLLIElement>('ul.blames li', byBlameEntrySk);

  if (!expectedBlames.length) {
    expect(noBlameList.innerText).to.contain('No blamelist.');
    expect(blames).to.be.empty;
  } else {
    expect(noBlameList).to.be.null;
    expect(blames).to.have.length(expectedBlames.length);

    for (let i = 0; i < expectedBlames.length; i++) {
      const linkText = $$<HTMLAnchorElement>('a', blames[i])!.innerText;
      const linkHref = $$<HTMLAnchorElement>('a', blames[i])!.href;
      const commitMessage = $$<HTMLElement>('.commit-message', blames[i])!.innerText;
      const author = $$<HTMLElement>('.author', blames[i])!.innerText;
      const age = $$<HTMLElement>('.age', blames[i])!.innerText;

      expect(linkText).to.contain(expectedBlames[i].linkText);
      expect(linkHref).to.equal(expectedBlames[i].linkHref);
      expect(commitMessage).to.contain(expectedBlames[i].commitMessage);
      expect(author).to.contain(expectedBlames[i].author);
      expect(age).to.contain(expectedBlames[i].age);
    }
  }
};

const expectNumTestsAffectedEquals = (byBlameEntrySk: ByBlameEntrySk, numTestsAffected: string) => expect($$<HTMLElement>('.num-tests-affected', byBlameEntrySk)!.innerText)
  .to.contain(numTestsAffected);

interface ExpectedRow {
  test: string;
  corpus: string;
  numDigests: number;
  exampleLinkText: string;
  exampleLinkHref: string;
}

const expectAffectedTestsTableEquals = (byBlameEntrySk: ByBlameEntrySk, expectedRows: ExpectedRow[]) => {
  const actualRows = $('.affected-tests tbody tr');
  expect(actualRows.length).to.equal(expectedRows.length);

  for (let i = 0; i < expectedRows.length; i++) {
    const test = $$<HTMLElement>('.test', actualRows[i])!.innerText;
    const corpus = $$<HTMLElement>('.corpus', actualRows[i])!.innerText;
    const numDigests = +$$<HTMLElement>('.num-digests', actualRows[i])!.innerText;
    const exampleLinkText = $$<HTMLAnchorElement>('a.example-link', actualRows[i])!.innerText;
    const exampleLinkHref = $$<HTMLAnchorElement>('a.example-link', actualRows[i])!.href;

    expect(test).to.contain(expectedRows[i].test);
    expect(corpus).to.contain(expectedRows[i].corpus);
    expect(numDigests).to.equal(expectedRows[i].numDigests);
    expect(exampleLinkText).to.contain(expectedRows[i].exampleLinkText);
    expect(exampleLinkHref).to.contain(expectedRows[i].exampleLinkHref);
  }
};
