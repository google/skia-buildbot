import './index.js';
import { entry, fakeNow, gitLog } from './test_data';

Date.now = () => fakeNow;

const byBlameEntrySk = document.createElement('byblameentry-sk');
byBlameEntrySk.byBlameEntry = entry;
byBlameEntrySk.gitLog = gitLog;
byBlameEntrySk.baseRepoUrl = 'https://skia.googlesource.com/skia.git';
byBlameEntrySk.corpus = 'gm';
document.body.appendChild(byBlameEntrySk);
