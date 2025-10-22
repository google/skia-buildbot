import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { DetailsDialogSk } from './details-dialog-sk';

import { expectLinks, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { comment, commit, commitsByHash, task, taskWithCustomExecutor } from './test_data';
import { SetTestSettings } from '../settings';
import { taskDriverData } from '../../../infra-sk/modules/task-driver-sk/test_data';

describe('details-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<DetailsDialogSk>('details-dialog-sk');

  let element: DetailsDialogSk;
  beforeEach(() => {
    SetTestSettings({
      swarmingUrl: 'https://example-swarming.appspot.com',
      treeStatusBaseUrl: 'https://example.com/treestatus',
      logsUrlTemplate:
        'https://ci.chromium.org/raw/build/logs.chromium.org/{{LogsProject}}/{{TaskID}}/+/annotations',
      taskSchedulerUrl: 'https://example.com/ts',
      defaultRepo: 'skia',
      repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
      repoToProject: new Map([['skia', 'skia']]),
    });
    element = newInstance((el: DetailsDialogSk) => {
      el.repo = 'skia';
    });
  });

  it('displays tasks', () => {
    element.displayTask(task, [comment], commitsByHash);
    expectLinks(element, [
      'https://ci.chromium.org/raw/build/logs.chromium.org/skia/1234561/+/annotations',
      'https://example.com/ts/task/999990',
      'https://example-swarming.appspot.com/task?id=1234560',
      'https://example-swarming.appspot.com/tasklist?f=sk_name%3ABuild-Some-Stuff',
      'https://skia.googlesource.com/skia/+show/abc123',
      'https://skia.googlesource.com/skia/+show/parentofabc123',
    ]);
    expect($$('button.action', element)).to.have.property('innerText', 'Re-run Job');
    expect($('.task-failure', element)).to.have.length(1);
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(1);
    expect($('table.blamelist tr', element)).to.have.length(2);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });

  it('displays tasks with taskExecutor', () => {
    element.displayTask(taskWithCustomExecutor, [comment], commitsByHash);
    expectLinks(element, [
      'https://ci.chromium.org/raw/build/logs.chromium.org/skia/1234561/+/annotations',
      'https://example.com/ts/task/999990',
      'https://some-other-swarming.appspot.com/task?id=1234560',
      'https://some-other-swarming.appspot.com/tasklist?f=sk_name%3ABuild-Some-Stuff',
      'https://skia.googlesource.com/skia/+show/abc123',
      'https://skia.googlesource.com/skia/+show/parentofabc123',
    ]);
  });

  it('displays tasks with taskdriver', async () => {
    fetchMock.getOnce('path:/json/td/999990', taskDriverData);
    element.displayTask(task, [comment], commitsByHash);
    await fetchMock.flush(true);

    expect($$('button.action', element)).to.have.property('innerText', 'Re-run Job');
    // No simple title with status, we have the task-driver-sk instead.
    expect($('.task-failure', element)).to.have.length(0);
    expect($('task-driver-sk', element)).to.have.length(1);
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(1);
    expect($('table.blamelist tr', element)).to.have.length(2);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });

  it('displays taskspec', () => {
    element.displayTaskSpec('', 'Build-Some-Thing', [comment]);
    expect($('button.action', element)).to.have.length(0);
    // 2 sections, seperated by a line.
    expect($('hr', element)).to.have.length(0);
    expect($('table.comments tr.comment', element)).to.have.length(1);

    expectLinks(element, [
      'https://example-swarming.appspot.com/tasklist?f=sk_name%3ABuild-Some-Thing',
    ]);
  });

  it('displays taskspec with taskExecutor', () => {
    element.displayTaskSpec('some-other-swarming.appspot.com', 'Build-Some-Thing', [comment]);
    expectLinks(element, [
      'https://some-other-swarming.appspot.com/tasklist?f=sk_name%3ABuild-Some-Thing',
    ]);
  });

  it('displays commit', () => {
    element.displayCommit(commit, [comment]);
    expect($$('button.action', element)).to.have.property('innerText', 'Revert');
    // 3 sections, seperated by lines.
    expect($('hr', element)).to.have.length(1);
    expect($('table.comments tr.comment', element)).to.have.length(1);
  });
});
