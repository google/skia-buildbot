import './index.js'
import { $, $$ } from 'common-sk/modules/dom'
import { deepCopy } from 'common-sk/modules/object'
import { byBlameEntry, gitLog } from './test_data'

describe('byblameentry-sk', () => {
  let byBlameEntrySk;

  function newByBlameEntrySk(
      byBlameEntry,
      {
        testGitLog = gitLog,
        baseRepoUrl = 'https://skia.googlesource.com/skia.git',
        corpus = 'gm'
      } = {}) {
    byBlameEntrySk = document.createElement('byblameentry-sk');
    byBlameEntrySk.byBlameEntry = byBlameEntry;
    byBlameEntrySk.gitLog = testGitLog;
    byBlameEntrySk.baseRepoUrl = baseRepoUrl;
    byBlameEntrySk.corpus = corpus;
    document.body.appendChild(byBlameEntrySk);
  }

  let clock;

  beforeEach(() => {
    // This is necessary to make commit ages deterministic, and is set to 50
    // seconds after Elisa's commit in the 'full example' test case below.
    clock = sinon.useFakeTimers(1573149864000); // Nov 7, 2019 6:04:24 PM GMT
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (byBlameEntrySk) {
      document.body.removeChild(byBlameEntrySk);
      byBlameEntrySk = null;
    }
    clock.restore();
  });

  describe('full example', () => {
    it('renders correctly', async () => {
      // This is a comprehensive example of a blame group with multiple
      // untriaged digests that could have originated from two different
      // commits, and includes a list of affected tests along with links to an
      // example untriaged digest for each affected test.

      await newByBlameEntrySk(byBlameEntry);
      expectTriageLinkEquals(
          '112 untriaged digests',
          // This server-generated blame ID is a colon-separated list of the
          // commit hashes blamed for these untriaged digests.
          '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&unt=true&head=true&query=source_type%3Dgm');
      expectBlamesListEquals([{
        linkText: 'bbbbbbb',
        linkHref: 'https://skia.googlesource.com/skia.git/+/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        commitMessage: 'One glyph() to rule them all!!!',
        author: 'Elisa (elisa@example.com)',
        age: '6h'
      }, {
        linkText: 'aaaaaaa',
        linkHref: 'https://skia.googlesource.com/skia.git/+/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        commitMessage: 'flesh out blendmodes through Screen',
        author: 'Joe (joe@example.com)',
        age: '5m'
      }]);
      expectNumTestsAffectedEquals('7 tests affected.');
      expectAffectedTestsTableEquals([{
        test: 'aarectmodes',
        numDigests: 50,
        exampleLinkText: 'c6476baec94eb6a5071606575318e4df',
        exampleLinkHref: '/detail?test=aarectmodes&digest=c6476baec94eb6a5071606575318e4df',
      }, {
        test: 'aaxfermodes',
        numDigests: 32,
        exampleLinkText: '4acfd6b3a3943cc5d75cd22e966ae6f1',
        exampleLinkHref: '/detail?test=aaxfermodes&digest=4acfd6b3a3943cc5d75cd22e966ae6f1',
      }, {
        test: 'hairmodes',
        numDigests: 21,
        exampleLinkText: 'f9e20c63b5ce3b58d9b6a90fa3e7224c',
        exampleLinkHref: '/detail?test=hairmodes&digest=f9e20c63b5ce3b58d9b6a90fa3e7224c',
      }, {
        test: 'imagefilters_xfermodes',
        numDigests: 5,
        exampleLinkText: '47644613317040264fea6fa815af32e8',
        exampleLinkHref: '/detail?test=imagefilters_xfermodes&digest=47644613317040264fea6fa815af32e8',
      }, {
        test: 'lattice2',
        numDigests: 2,
        exampleLinkText: '16e41798ecd59b1523322a57b49cc17f',
        exampleLinkHref: '/detail?test=lattice2&digest=16e41798ecd59b1523322a57b49cc17f',
      }, {
        test: 'xfermodes',
        numDigests: 1,
        exampleLinkText: '8fbee03f794c455c4e4842ec2736b744',
        exampleLinkHref: '/detail?test=xfermodes&digest=8fbee03f794c455c4e4842ec2736b744',
      }, {
        test: 'xfermodes3',
        numDigests: 1,
        exampleLinkText: 'fed2ff29abe371fc0ec1b2c65dfb3949',
        exampleLinkHref: '/detail?test=xfermodes3&digest=fed2ff29abe371fc0ec1b2c65dfb3949',
      }]);
    });
  });

  describe('triage link', () => {
    it('renders link text in singular if there is just 1 digest', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.nDigests = 1;
      await newByBlameEntrySk(testByBlameEntry);
      expectTriageLinkEquals(
          '1 untriaged digest',
          '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&unt=true&head=true&query=source_type%3Dgm');
    });

    it('includes the right corpus in the link href', async () => {
      await newByBlameEntrySk(byBlameEntry, {corpus: 'svg'});
      expectTriageLinkEquals(
          '112 untriaged digests',
          '/search?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&unt=true&head=true&query=source_type%3Dsvg');
    })
  });

  describe('blamelist', () => {
    it('shows "No blamelist" message if there are 0 blames', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.commits = [];
      await newByBlameEntrySk(testByBlameEntry);
      expectBlamesListEquals([]);
    });

    it('points commit links to GitHub if repo is hosted there', async () => {
      await newByBlameEntrySk(
          byBlameEntry,
          {baseRepoUrl: 'https://github.com/google/skia'});
      expectBlamesListEquals([{
        linkText: 'bbbbbbb',
        linkHref: 'https://github.com/google/skia/commit/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        commitMessage: 'One glyph() to rule them all!!!',
        author: 'Elisa (elisa@example.com)',
        age: '6h'
      }, {
        linkText: 'aaaaaaa',
        linkHref: 'https://github.com/google/skia/commit/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        commitMessage: 'flesh out blendmodes through Screen',
        author: 'Joe (joe@example.com)',
        age: '5m'
      }]);
    });

    it('shows empty commit messages if Git log is empty/missing', async () => {
      await newByBlameEntrySk(byBlameEntry, {testGitLog: {log: []}});
      expectBlamesListEquals([{
        linkText: 'bbbbbbb',
        linkHref: 'https://skia.googlesource.com/skia.git/+/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        commitMessage: '',
        author: 'Elisa (elisa@example.com)',
        age: '6h'
      }, {
        linkText: 'aaaaaaa',
        linkHref: 'https://skia.googlesource.com/skia.git/+/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        commitMessage: '',
        author: 'Joe (joe@example.com)',
        age: '5m'
      }]);
    })
  });

  describe('affected tests', () => {
    it('renders correctly with nTests = 0', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.nTests = 0;
      testByBlameEntry.affectedTests = [];
      await newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals('0 tests affected.');
      expectAffectedTestsTableEquals([]);
    });

    it('renders correctly with nTests = 1 and 0 affected tests', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.nTests = 1;
      testByBlameEntry.affectedTests = [];
      await newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals('1 test affected.');
      expectAffectedTestsTableEquals([]);
    });

    it('renders correctly with nTests = 2 and 0 affected tests', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.nTests = 2;
      testByBlameEntry.affectedTests = [];
      await newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals('2 tests affected.');
      expectAffectedTestsTableEquals([]);
    });

    it('renders correctly with nTests = 2 and one affected test', async () => {
      const testByBlameEntry = deepCopy(byBlameEntry);
      testByBlameEntry.nTests = 2;
      testByBlameEntry.affectedTests = [{
        "test": "aarectmodes",
        "num": 5,
        "sample_digest": "c6476baec94eb6a5071606575318e4df",
      }];
      await newByBlameEntrySk(testByBlameEntry);
      expectNumTestsAffectedEquals('2 tests affected.');
      expectAffectedTestsTableEquals([{
        test: 'aarectmodes',
        numDigests: 5,
        exampleLinkText: 'c6476baec94eb6a5071606575318e4df',
        exampleLinkHref:
            '/detail?test=aarectmodes&digest=c6476baec94eb6a5071606575318e4df',
      }]);
    });
  });

  const expectTriageLinkEquals = (text, href) => {
    const triageLink = $$('a.triage', byBlameEntrySk);
    expect(triageLink.innerText).to.contain(text);
    expect(triageLink.href).to.have.string(href);
  };

  const expectBlamesListEquals = (expectedBlames) => {
    const noBlameList = $$('p.no-blamelist', byBlameEntrySk);
    const blames = $('ul.blames li', byBlameEntrySk);

    if (!expectedBlames.length) {
      expect(noBlameList.innerText).to.contain('No blamelist.');
      expect(blames).to.be.empty;
    } else {
      expect(noBlameList).to.be.null;
      expect(blames).to.have.length(expectedBlames.length);

      for (let i = 0; i < expectedBlames.length; i++) {
        const linkText = $$('a', blames[i]).innerText;
        const linkHref = $$('a', blames[i]).href;
        const commitMessage =
            $$('.commit-message', blames[i]).innerText;
        const author = $$('.author', blames[i]).innerText;
        const age = $$('.age', blames[i]).innerText;

        expect(linkText).to.contain(expectedBlames[i].linkText);
        expect(linkHref).to.equal(expectedBlames[i].linkHref);
        expect(commitMessage).to.contain(expectedBlames[i].commitMessage);
        expect(author).to.contain(expectedBlames[i].author);
        expect(age).to.contain(expectedBlames[i].age);
      }
    }
  };

  const expectNumTestsAffectedEquals =
      (numTestsAffected) =>
          expect($$('.num-tests-affected', byBlameEntrySk).innerText)
              .to.contain(numTestsAffected);

  const expectAffectedTestsTableEquals = (expectedRows) => {
    const actualRows = $('.affected-tests tbody tr');
    expect(actualRows.length).to.equal(expectedRows.length);

    for (let i = 0; i < expectedRows.length; i++) {
      const test = $$('.test', actualRows[i]).innerText;
      const numDigests =
          +$$('.num-digests', actualRows[i]).innerText;
      const exampleLinkText = $$('a.example-link', actualRows[i]).innerText;
      const exampleLinkHref = $$('a.example-link', actualRows[i]).href;

      expect(test).to.contain(expectedRows[i].test);
      expect(numDigests).to.equal(expectedRows[i].numDigests);
      expect(exampleLinkText).to.contain(expectedRows[i].exampleLinkText);
      expect(exampleLinkHref).to.contain(expectedRows[i].exampleLinkHref);
    }
  };
});
