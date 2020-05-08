import './index';
import { entry, fakeNow, gitLog } from './test_data';
import { testOnlySetSettings } from '../settings';

Date.now = () => fakeNow;
testOnlySetSettings({
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

const byBlameEntrySk = document.createElement('byblameentry-sk');
byBlameEntrySk.byBlameEntry = entry;
byBlameEntrySk.gitLog = gitLog;
byBlameEntrySk.corpus = 'gm';
document.body.appendChild(byBlameEntrySk);
