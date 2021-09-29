import { ChangelistsResponse } from '../rpc_types';

export const fakeNow = Date.parse('2019-09-09T19:32:30Z');

export const changelistSummaries_5: ChangelistsResponse = {
  changelists: [
    {
      system: 'gerrit',
      id: '1788313',
      owner: 'alpha@example.org',
      status: 'Open',
      subject: '[omnibox] Add flag to preserve the default match against async updates',
      updated: '2019-09-09T19:31:14Z',
      url: 'https://chromium-review.googlesource.com/1788313',
    },
    {
      system: 'gerrit',
      id: '1787403',
      owner: 'beta-autoroll@example.iam.gserviceaccount.com',
      status: 'Open',
      subject: 'Convert AssociatedInterfacePtr to AssociatedRemote in chrome/b/android',
      updated: '2019-09-09T19:30:41Z',
      url: 'https://chromium-review.googlesource.com/1787403',
    },
    {
      system: 'gerrit',
      id: '1788259',
      owner: 'gamma@example.org',
      status: 'Open',
      subject:
          'Implement deep content compliance and malware scans for uploads. '
          + 'This is a really long subject, like wow! '
          + "I hope the web UI doesn't mishandle this massively long subject",
      updated: '2019-09-09T19:28:54Z',
      url: 'https://chromium-review.googlesource.com/1788259',
    },
    {
      system: 'gerrit',
      id: '1792459',
      owner: 'delta@example.org',
      status: 'Abandoned',
      subject: 'Remove unneeded SurfaceSync CHECK',
      updated: '2019-09-09T19:26:25Z',
      url: 'https://chromium-review.googlesource.com/1792459',
    },
    {
      system: 'gerrit',
      id: '1790066',
      owner: 'epsilon@example.com',
      status: 'Landed',
      subject: 'Performance improvement for ITextRangeProvider::GetEnclosingElement',
      updated: '2019-09-09T19:24:10Z',
      url: 'https://chromium-review.googlesource.com/1790066',
    },
  ],
  offset: 0,
  size: 5,
  total: 2147483647,
};

export const changelistSummaries_5_offset5 = {
  changelists: [
    {
      system: 'gerrit',
      id: '1806853',
      owner: 'zeta@example.org',
      status: 'Open',
      subject: 'Fix cursor in <pin-keyboard> on taps after keybrd',
      updated: '2019-09-09T18:12:34Z',
      url: 'https://chromium-review.googlesource.com/1806853',
    },
    {
      system: 'gerrit',
      id: '1790204',
      owner: 'eta@example.com',
      status: 'Open',
      subject: 'Replaced WrapUnique with make_unique in components/',
      updated: '2019-09-09T17:12:34Z',
      url: 'https://chromium-review.googlesource.com/1790204',
    },
    {
      system: 'gerrit',
      id: '1787402',
      owner: 'theta@example.org',
      status: 'Open',
      subject: 'Use an Omaha-style GUID app ID for updater self-updates.',
      updated: '2019-09-09T16:12:34Z',
      url: 'https://chromium-review.googlesource.com/1787402',
    },
    {
      system: 'gerrit',
      id: '1804242',
      owner: 'iota@example.org',
      status: 'Abandoned',
      subject: 'Register CUS paths on ios.',
      updated: '2019-09-09T15:12:34Z',
      url: 'https://chromium-review.googlesource.com/1804242',
    },
    {
      system: 'gerrit',
      id: '1804507',
      owner: 'kappa@example.com',
      status: 'Landed',
      subject: "PM: Don't use mojo for UKM no more.",
      updated: '2019-09-09T14:12:34Z',
      url: 'https://chromium-review.googlesource.com/1804507',
    },
  ],
  offset: 5,
  size: 5,
  total: 2147483647,
};

export const changelistSummaries_5_offset10: ChangelistsResponse = {
  changelists: [
    {
      system: 'gerrit',
      id: '1793168',
      owner: 'lambda@example.org',
      status: 'Open',
      subject: 'Validate scanned card number',
      updated: '2019-09-09T13:12:34Z',
      url: 'https://chromium-review.googlesource.com/1793168',
    },
    {
      system: 'gerrit',
      id: '1805865',
      owner: 'mu@example.com',
      status: 'Open',
      subject: 'Update SkiaRenderer BrowserTests Filter',
      updated: '2019-09-09T12:12:34Z',
      url: 'https://chromium-review.googlesource.com/1805865',
    },
    {
      system: 'gerrit',
      id: '1798703',
      owner: 'nu@example.org',
      status: 'Open',
      subject: '[IOS] Pass dispatcher to CardMediator',
      updated: '2019-09-09T11:12:34Z',
      url: 'https://chromium-review.googlesource.com/1798703',
    },
    {
      system: 'gerrit',
      id: '1805862',
      owner: 'xi@example.org',
      status: 'Abandoned',
      subject: 'Updating XTBs based on .GRDs from branch master',
      updated: '2019-09-09T10:12:34Z',
      url: 'https://chromium-review.googlesource.com/1805862',
    },
    {
      system: 'gerrit',
      id: '1805646',
      owner: 'omicron@example.com',
      status: 'Landed',
      subject: '[Sheriff] Disable UkmBrowserTest.EvictObsoleteSources',
      updated: '2019-09-09T09:12:34Z',
      url: 'https://chromium-review.googlesource.com/1805646',
    },
  ],
  offset: 10,
  size: 5,
  total: 2147483647,
};

export const empty = (partial?: Partial<ChangelistsResponse>): ChangelistsResponse => ({
  changelists: null,
  offset: partial?.offset || 0,
  size: partial?.size || 5,
  total: partial?.total || 2147483647,
});
