import './index';
import { entry, fakeNow } from './test_data';
import { testOnlySetSettings } from '../settings';
import { ByBlameEntrySk } from './byblameentry-sk';

Date.now = () => fakeNow;
testOnlySetSettings({
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

const byBlameEntrySk = new ByBlameEntrySk();
byBlameEntrySk.byBlameEntry = entry;
byBlameEntrySk.corpus = 'gm';
document.body.appendChild(byBlameEntrySk);
