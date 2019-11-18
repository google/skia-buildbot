import './index.js'
import '../gold-scaffold-sk'
import { $$ } from 'common-sk/modules/dom'
import { byBlameEntry, gitLog } from './test_data'

const entry = document.createElement('byblameentry-sk');
entry.byBlameEntry = byBlameEntry;
entry.gitLog = gitLog;
entry.baseRepoUrl = 'https://skia.googlesource.com/skia.git';
entry.corpus = 'gm';
$$('gold-scaffold-sk').appendChild(entry);
