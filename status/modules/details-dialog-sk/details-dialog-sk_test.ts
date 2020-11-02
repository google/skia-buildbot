import './index';
import { DetailsDialogSk } from './details-dialog-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { comment, commit, commitsByHash, task } from './test_data';
import fetchMock from 'fetch-mock';
import { SetTestSettings } from '../settings';
import { $, $$ } from 'common-sk/modules/dom';
import { taskDriverData } from '../../../infra-sk/modules/task-driver-sk/test_data';

describe('details-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<DetailsDialogSk>('details-dialog-sk');

  let element: DetailsDialogSk;
  beforeEach(() => {
    SetTestSettings({
      swarmingUrl: 'example.com/swarming',
      logsUrlTemplate:
        'https://ci.chromium.org/raw/build/logs.chromium.org/skia/{{TaskID}}/+/annotations',
      taskSchedulerUrl: 'example.com/ts',
      defaultRepo: 'skia',
      repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
    });
    element = newInstance((el: DetailsDialogSk) => {
      el.repo = 'skia';
    });
  });

  it('displays tasks', () => {
    element.displayTask(task, [comment], commitsByHash);
    expect($$<HTMLAnchorElement>('a', element)!.href).to.equal(
      'https://ci.chromium.org/raw/build/logs.chromium.org/skia/1234561/+/annotations'
    );
    expect($$('button.action', element)).to.have.property('innerText', 'Re-run Job');
    expect($('.task-failure', element)).to.have.length(1);
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(2);
    expect($('table.blamelist tr', element)).to.have.length(2);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });

  it('displays tasks with taskdriver', async () => {
    fetchMock.getOnce('path:/json/td/99999', taskDriverData);
    element.displayTask(task, [comment], commitsByHash);
    await fetchMock.flush(true);

    expect($$('button.action', element)).to.have.property('innerText', 'Re-run Job');
    // No simple title with status, we have the task-driver-sk instead.
    expect($('.task-failure', element)).to.have.length(0);
    expect($('task-driver-sk', element)).to.have.length(1);
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(2);
    expect($('table.blamelist tr', element)).to.have.length(2);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });

  it('displays taskspec', () => {
    element.displayTaskSpec('Build-Some-Thing', [comment]);
    expect($('button.action', element)).to.have.length(0);
    // 2 sections, seperated by a line.
    expect($('hr', element)).to.have.length(1);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });

  it('displays commit', () => {
    element.displayCommit(commit, [comment]);
    expect($$('button.action', element)).to.have.property('innerText', 'Revert');
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(2);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });
});
