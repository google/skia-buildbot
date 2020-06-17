import './index';
import { $$ } from 'common-sk/modules/dom';
import { BlamelistPanelSk } from './blamelist-panel-sk';
import { blamelist19, clBlamelist, fakeNow } from './demo_data';
import { testOnlySetSettings } from '../settings';

Date.now = () => fakeNow;

testOnlySetSettings({
  baseRepoURL: 'https://github.com/example/example',
  crsTemplate: 'https://skia-review.googlesource.com/%s',
});

let ele = new BlamelistPanelSk();
ele.commits = blamelist19.slice(0, 2);
$$('#single_commit')!.appendChild(ele);

ele = new BlamelistPanelSk();
ele.commits = clBlamelist;
$$('#single_cl_commit')!.appendChild(ele);

ele = new BlamelistPanelSk();
ele.commits = blamelist19.slice(0, 4);
$$('#some_commits')!.appendChild(ele);

ele = new BlamelistPanelSk();
ele.commits = blamelist19;
$$('#many_commits')!.appendChild(ele);
