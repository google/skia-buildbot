import './index.js';
import { byBlameEntry, fakeNow, gitLog } from './test_data';

Date.now = () => fakeNow;

const entry = document.createElement('byblameentry-sk');
entry.byBlameEntry = byBlameEntry;
entry.gitLog = gitLog;
entry.baseRepoUrl = 'https://skia.googlesource.com/skia.git';
entry.corpus = 'gm';
document.body.appendChild(entry);
