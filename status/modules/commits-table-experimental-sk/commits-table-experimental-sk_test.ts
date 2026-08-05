import './index';

import { expect } from 'chai';
import { $, $$ } from '../../../infra-sk/modules/dom';
import {
  setUpElementUnderTest,
  eventPromise,
  setQueryString,
} from '../../../infra-sk/modules/test_util';
import {
  incrementalResponse0,
  responseMultiCommitTask,
  responseNoncontiguousCommitsTask,
  responseTasksToFilter,
  branch0,
  branch1,
  commentCommit,
  commentTask,
  commentTaskSpec,
  incrementalResponse1,
  resetResponse0,
} from '../rpc-mock/test_data';
import { GetIncrementalCommitsRequest, GetIncrementalCommitsResponse } from '../rpc';
import { CommitsTableExperimentalSk } from './commits-table-experimental-sk';
import { MockStatusService, SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';

describe('commits-table-experimental-sk', () => {
  const newTableInstance = setUpElementUnderTest('commits-table-experimental-sk');
  SetTestSettings({
    swarmingUrl: 'https://example-swarming.appspot.com',
    treeStatusBaseUrl: 'https://example.com/treestatus',
    logsUrlTemplate:
      'https://ci.chromium.org/raw/build/logs.chromium.org/{{LogsProject}}/{{TaskID}}/+/annotations',
    taskSchedulerUrl: 'https://example.com/ts',
    defaultRepo: 'skia',
    repos: new Map([
      ['skia', 'https://skia.googlesource.com/skia/+show/'],
      ['infra', 'https://skia.googlesource.com/buildbot/+show/'],
      ['skcms', 'https://skia.googlesource.com/skcms/+show/'],
    ]),
    repoToProject: new Map([
      ['skia', 'skia'],
      ['infra', 'skiabuildbot'],
      ['skcms', 'skcms'],
    ]),
  });

  let mocks: MockStatusService;
  beforeEach(async () => {
    mocks = SetupMocks();
    // Clear Url between tests.
    window.history.replaceState(null, '', window.location.origin + window.location.pathname);
  });

  afterEach(async () => {
    expect(mocks.exhausted()).to.equal(true);
  });

  const setupWithResponse = async (
    resp: GetIncrementalCommitsResponse,
    validator?: (req: GetIncrementalCommitsRequest) => void
  ) => {
    mocks.expectGetIncrementalCommits(resp, validator);
    const ep = eventPromise('end-task');
    const table = newTableInstance(
      (el) => ((<CommitsTableExperimentalSk>el).filter = 'All')
    ) as CommitsTableExperimentalSk;
    await ep;
    return table;
  };

  it('displays multiple commit tasks', async () => {
    const table = await setupWithResponse(responseMultiCommitTask);
    expect($('.task', table)).to.have.length(1);
    expect($$('.task', table)!.classList.value).to.include('bg-failure');
  });

  it('displays noncontiguous tasks', async () => {
    const table = await setupWithResponse(responseNoncontiguousCommitsTask);
    expect($('.multicommit-task', table)).to.have.length(1);
    const multicommitDiv = $$('.multicommit-task', table)!;
    // Parent div holds one div per commit, and one for the gap.
    expect($('.task', multicommitDiv)).to.have.length(3);
    expect($('.task.dashed-bottom', multicommitDiv)).to.have.length(1);
    expect($('.task.hidden', multicommitDiv)).to.have.length(1);
    expect($('.task.dashed-top', multicommitDiv)).to.have.length(1);
  });

  it('displays commits', async () => {
    const table = await setupWithResponse(incrementalResponse0);
    const commitDivs = $('.commit', table);
    expect(commitDivs).to.have.length(5);
    // The commit divs, when sorted by vertical position, match the order of the original commits.
    expect(
      commitDivs
        .sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)
        // Get hash from class list.
        .map((el) => el.classList.item(1))
    ).to.deep.equal(incrementalResponse0.update!.commits!.map((c) => `commit-${c.hash}`));
  });

  it('displays icons', async () => {
    const table = await setupWithResponse(incrementalResponse0);
    expect($('.tasksTable comment-icon-sk', table)).to.have.length(3);
    expect($('.commit-parentofabc123.commit comment-icon-sk', table)).to.have.length(1);
    expect($('.commit-parentofabc123.commit block-icon-sk', table)).to.have.length(1);
    expect($('.task-spec[title="Build-Some-Stuff"] comment-icon-sk', table)).to.have.length(1);
    expect($('.task[title="Build-Some-Stuff @ abc123"] comment-icon-sk', table)).to.have.length(1);
  });

  it('highlights reverts/relands', async () => {
    const table = await setupWithResponse(incrementalResponse0);
    expect($('.commit-bad.commit undo-icon-sk', table)).to.have.length(1);

    const revertedCommitDiv = $$('.commit-1revertbad.commit', table)!;
    $$('.commit-bad.commit undo-icon-sk', table)!.dispatchEvent(new Event('mouseenter', {}));
    expect(revertedCommitDiv.classList.value).to.include('highlight-revert');
    $$('.commit-bad.commit undo-icon-sk', table)!.dispatchEvent(new Event('mouseleave', {}));
    expect(revertedCommitDiv.classList.value).to.not.include('highlight-revert');

    const relandCommitDiv = $$('.commit-relandbad.commit', table)!;
    $$('.commit-bad.commit redo-icon-sk', table)!.dispatchEvent(new Event('mouseenter', {}));
    expect(relandCommitDiv.classList.value).to.include('highlight-reland');
    $$('.commit-bad.commit redo-icon-sk', table)!.dispatchEvent(new Event('mouseleave', {}));
    expect(relandCommitDiv.classList.value).to.not.include('highlight-reland');
  });

  it('handles mouseenter/mouseleave on tasks with missing commits gracefully', async () => {
    const table = await setupWithResponse(incrementalResponse0);
    const mockTask = {
      id: 'mockTask',
      name: 'Mock-Task',
      commits: ['abc123', 'missingcommit'],
      status: 'FAILURE',
    };
    const commitDiv = $$('.commit-abc123', table)!;

    // Trigger taskMouseInOut directly, which should not throw an exception.
    expect(() => {
      (table as any).taskMouseInOut(mockTask);
    }).to.not.throw();
    // It should have toggled the class on the visible commit div.
    expect(commitDiv.classList.value).to.include('task-emphasize-failure');

    expect(() => {
      (table as any).taskMouseInOut(mockTask);
    }).to.not.throw();
    // It should have untoggled the class on the visible commit div.
    expect(commitDiv.classList.value).to.not.include('task-emphasize-failure');
  });

  it('displays task boxes correctly for tasks with commits outside the window', async () => {
    const copy = <T>(x: T): T => JSON.parse(JSON.stringify(x));
    const r = copy(responseNoncontiguousCommitsTask);

    // Customize the commits and task
    // We add commit_below and commit_below_below to the bottom of the window.
    const commit_below = {
      hash: '789012',
      author: 'charles@example.com',
      parents: ['901234'],
      subject: 'older commit',
      body: 'older commit',
      timestamp: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
    };
    const commit_below_below = {
      hash: '901234',
      author: 'dave@example.com',
      parents: ['876543'],
      subject: 'even older commit',
      body: 'even older commit',
      timestamp: new Date(Date.now() - 20 * 60 * 1000).toISOString(),
    };

    // The parent of commit1 ('parentofabc123') needs to be '789012' (commit_below)
    // r.update!.commits has [commit0, branchCommit0, commit1].
    r.update!.commits![2].parents = [commit_below.hash];
    r.update!.commits!.push(commit_below, commit_below_below);

    // Now let's set the task commits. 'unseen_newer_commit' is unseen.
    // 'abc123' is visible commit 0, 'parentofabc123' is visible commit 2.
    r.update!.tasks = [
      {
        commits: ['unseen_newer_commit', 'abc123', 'parentofabc123'],
        id: '99999',
        name: 'Build-Some-Stuff',
        revision: 'abc123',
        status: 'FAILURE',
        swarmingTaskId: 'swarmy0',
        taskExecutor: '',
      },
    ];

    const table = await setupWithResponse(r);
    expect($('.multicommit-task', table)).to.have.length(1);
    const multicommitDiv = $$('.multicommit-task', table)!;

    // We expect the multicommit task to only span:
    // 1. abc123 (task commit) -> true
    // 2. 456789 (branch commit) -> false (gap)
    // 3. parentofabc123 (task commit) -> true
    // Total: 3 divs inside multicommit-task.
    // If it incorrectly stretched to the bottom of the window, it would also include a gap
    // for commit_below_below, resulting in 4 or 5 divs inside multicommit-task.
    expect($('.task', multicommitDiv)).to.have.length(3);

    // Specifically verify the structure of displayTaskRows output
    const taskRows = (table as any).displayTaskRows(r.update!.tasks[0], 0);
    expect(taskRows).to.deep.equal([true, false, true]);
  });

  it('filters task specs', async () => {
    const table = await setupWithResponse(responseTasksToFilter);
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Green-Spec',
      'Always-Red-Spec',
      'Interesting-Spec',
      'Only-Failed-On-Commented-Commit-Spec',
    ]);

    const clickLabel = (i: number, expectText: string) => {
      const label = $('tabs-sk button', table)[i] as HTMLLabelElement;
      expect(label.innerText).to.contain(expectText);
      label.click();
    };
    clickLabel(0, 'Interesting');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Interesting-Spec',
    ]);

    clickLabel(1, 'Failures');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Red-Spec',
      'Interesting-Spec',
      'Only-Failed-On-Commented-Commit-Spec',
    ]);

    clickLabel(2, 'Comments');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Red-Spec',
    ]);
    clickLabel(3, 'Failing w/o comment');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Interesting-Spec',
    ]);

    const searchbox = $$('.controls input-sk input', table) as HTMLInputElement;
    searchbox.value = 'Always';
    const ep = eventPromise('change');
    searchbox.dispatchEvent(new Event('change', { bubbles: true }));
    await ep;
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Green-Spec',
      'Always-Red-Spec',
    ]);
  });

  it('sorts columns alphabetically', async () => {
    const resp = JSON.parse(JSON.stringify(incrementalResponse0));
    resp.update.tasks = [
      { name: 'Z-Category-Task', status: 'SUCCESS', commits: ['abc123'], id: 'taskZ' },
      { name: 'A-Category-Task', status: 'SUCCESS', commits: ['abc123'], id: 'taskA' },
      { name: 'M-Category-Task', status: 'SUCCESS', commits: ['abc123'], id: 'taskM' },
    ];
    const table = await setupWithResponse(resp);
    const taskSpecs = $('.task-spec', table).map((el) => el.getAttribute('title'));
    expect(taskSpecs).to.deep.equal(['A-Category-Task', 'M-Category-Task', 'Z-Category-Task']);

    const categories = $('.category:not(.task-spec)', table).map((el) =>
      (el as HTMLElement).innerText.trim()
    );
    // Categories and subcategories should also be sorted.
    // Based on split('-'), 'A-Category-Task' has category 'A', subcategory 'Category'.
    expect(categories).to.include('A');
    expect(categories).to.include('M');
    expect(categories).to.include('Z');

    // Verify the relative order of categories in the DOM.
    // They are rendered in the order they appear in the categories map.
    const categoryDivs = $('.category:not(.task-spec)', table)
      .filter((el) => !(el as HTMLElement).innerText.includes('Category')) // Filter out subcategories
      .map((el) => (el as HTMLElement).innerText.trim());
    expect(categoryDivs).to.deep.equal(['A', 'M', 'Z']);
  });

  it('incorporates incremental update', async () => {
    const mocker = SetupMocks().expectGetIncrementalCommits(incrementalResponse0);
    const ep = eventPromise('end-task');
    const table = newTableInstance(
      (el) => ((<CommitsTableExperimentalSk>el).filter = 'All')
    ) as CommitsTableExperimentalSk;
    await ep;
    let commitDivs = $('.commit', table);
    expect(commitDivs).to.have.length(5);
    expect($('.task[title="Test-Some-Stuff @ parentofabc123"]', table)).to.have.length(1);
    expect(
      $$('.task[title="Test-Some-Stuff @ parentofabc123"]', table)?.classList.value
    ).to.contain('bg-failure');
    // Mock an incremental update, and change the commits input to trigger it.
    mocker.expectGetIncrementalCommits(incrementalResponse1);
    const commitsInput = $$('#commitsInput input', table) as HTMLInputElement;
    commitsInput.dispatchEvent(new Event('change', { bubbles: true }));
    // eventPromise for the same event 'end-task' seems to instantly resolve, so hack the delay.
    await new Promise((resolve) => setTimeout(resolve, 0));

    commitDivs = $('.commit', table);
    expect(commitDivs).to.have.length(6);
    // The commit divs, when sorted by vertical position, match the order of new commits followed
    // by the the original commits.
    expect(
      commitDivs
        .sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)
        // Get hash from class list.
        .map((el) => el.classList.item(1))
    ).to.deep.equal(
      incrementalResponse1
        .update!.commits!.map((c) => `commit-${c.hash}`)
        .concat(incrementalResponse0.update!.commits!.map((c) => `commit-${c.hash}`))
    );

    // New task is present.
    expect($('.task[title="Build-Some-Stuff @ childofabc123"]', table)).to.have.length(1);
    // Old task is updated.
    expect(
      $$('.task[title="Test-Some-Stuff @ parentofabc123"]', table)?.classList.value
    ).to.contain('bg-success');
  });

  it('resets with startOver update', async () => {
    const mocker = SetupMocks().expectGetIncrementalCommits(incrementalResponse0);
    const ep = eventPromise('end-task');
    const table = newTableInstance(
      (el) => ((<CommitsTableExperimentalSk>el).filter = 'All')
    ) as CommitsTableExperimentalSk;
    await ep;
    let commitDivs = $('.commit', table);
    expect(commitDivs).to.have.length(5);
    // Mock an incremental update, and change the commits input to trigger it.
    mocker.expectGetIncrementalCommits(resetResponse0);
    const commitsInput = $$('#commitsInput input', table) as HTMLInputElement;
    commitsInput.dispatchEvent(new Event('change', { bubbles: true }));
    // eventPromise for the same event 'end-task' seems to instantly resolve, so hack the delay.
    await new Promise((resolve) => setTimeout(resolve, 0));

    commitDivs = $('.commit', table);
    expect(commitDivs).to.have.length(1);
    // Only the new commit and it's single task are present.
    expect(commitDivs[0].classList.toString()).to.contain(resetResponse0.update!.commits![0].hash);
    expect($('.task[title="Build-Some-Stuff @ childofabc123"]', table)).to.have.length(1);
  });

  it('initial request uses repo from query string', async () => {
    setQueryString('?repo=infra');
    await setupWithResponse(responseMultiCommitTask, (req: GetIncrementalCommitsRequest) => {
      expect(req.repoPath).to.equal('infra');
    });
  });

  it('filters commits by branch name', async () => {
    const branch_main = { name: 'main', head: 'abc123' };
    const branch_feature = { name: 'feature', head: 'xyz789' };
    const commit_main = {
      hash: 'abc123',
      author: 'alice@example.com',
      parents: ['parent1'],
      subject: 'commit main',
      body: 'commit main',
      timestamp: new Date(Date.now() - 1000).toISOString(),
    };
    const commit_feature = {
      hash: 'xyz789',
      author: 'bob@example.com',
      parents: ['parent1'],
      subject: 'commit feature',
      body: 'commit feature',
      timestamp: new Date(Date.now() - 2000).toISOString(),
    };
    const commit_parent = {
      hash: 'parent1',
      author: 'charlie@example.com',
      parents: [],
      subject: 'commit parent',
      body: 'commit parent',
      timestamp: new Date(Date.now() - 3000).toISOString(),
    };
    const resp: GetIncrementalCommitsResponse = {
      metadata: { pod: 'podd', startOver: true },
      update: {
        branchHeads: [branch_main, branch_feature],
        commits: [commit_main, commit_feature, commit_parent],
        tasks: [],
      },
    };

    const table = await setupWithResponse(resp);
    // At first, all 3 commits are displayed.
    expect($('.commit', table)).to.have.length(3);

    // Filter by branch 'feature'
    table.branchFilter = 'feature';
    // Now only commit_feature ('xyz789') and commit_parent ('parent1') should be shown.
    expect($('.commit', table)).to.have.length(2);
    expect($('.commit-xyz789', table)).to.have.length(1);
    expect($('.commit-parent1', table)).to.have.length(1);
    expect($('.commit-abc123', table)).to.have.length(0);

    // Filter by branch 'main'
    table.branchFilter = 'main';
    // Now only commit_main ('abc123') and commit_parent ('parent1') should be shown.
    expect($('.commit', table)).to.have.length(2);
    expect($('.commit-abc123', table)).to.have.length(1);
    expect($('.commit-parent1', table)).to.have.length(1);
    expect($('.commit-xyz789', table)).to.have.length(0);

    // Filter by invalid regex
    table.branchFilter = 'non-existent';
    expect($('.commit', table)).to.have.length(0);

    // Clear filter
    table.branchFilter = '';
    expect($('.commit', table)).to.have.length(3);
  });

  describe('dialog', () => {
    it('opens and closes properly', async () => {
      const table = await setupWithResponse(incrementalResponse0);
      expect($$('details-dialog-sk', table)).to.have.nested.property('style.display', '');
      (<HTMLDivElement>$$('[data-task-id="99999"]', table)).click();
      expect($$('details-dialog-sk', table)).to.have.nested.property('style.display', 'block');
      // Clicking somewhere in the dialog doesn't close it.
      (<HTMLTableCellElement>$$('details-dialog-sk td', table)).click();
      expect($$('details-dialog-sk', table)).to.have.nested.property('style.display', 'block');
      // Clicking elsewhere does close it.
      (<HTMLDivElement>$$('div.tasksTable', table)).click();
      expect($$('details-dialog-sk', table)).to.have.nested.property('style.display', 'none');
    });

    it('displays tasks', async () => {
      const table = await setupWithResponse(incrementalResponse0);
      expect($('[data-task-id="99999"]', table)).to.have.length(1);
      (<HTMLDivElement>$$('[data-task-id="99999"]', table)).click();
      expect($$('details-dialog-sk .dialog h3', table)).to.have.property(
        'innerText',
        'Build-Some-Stuff'
      );
      expect($$('details-dialog-sk .dialog h3 a', table)).to.have.property(
        'href',
        'https://ci.chromium.org/raw/build/logs.chromium.org/skia/swarmy1/+/annotations'
      );
      expect($('details-dialog-sk .dialog table.blamelist tr', table)).to.have.length(1);
      expect($('details-dialog-sk .dialog table.comments tr.comment', table)).to.have.length(1);
    });

    it('displays taskSpecs', async () => {
      const table = await setupWithResponse(incrementalResponse0);
      expect($('[title="Build-Some-Stuff"]', table)).to.have.length(1);
      (<HTMLDivElement>$$('[title="Build-Some-Stuff"]', table)).click();
      expect($$('details-dialog-sk .dialog h3', table)).to.have.property(
        'innerText',
        'Build-Some-Stuff'
      );
      expect($('details-dialog-sk .dialog table.comments tr.comment', table)).to.have.length(1);
      expect($$('details-dialog-sk .dialog h3 a', table)).to.have.property(
        'href',
        'https://example-swarming.appspot.com/tasklist?f=sk_name%3ABuild-Some-Stuff'
      );
    });

    it('displays taskSpecs with taskExecutor', async () => {
      const table = await setupWithResponse(incrementalResponse0);
      expect($('[title="Test-Some-Stuff"]', table)).to.have.length(1);
      (<HTMLDivElement>$$('[title="Test-Some-Stuff"]', table)).click();
      expect($$('details-dialog-sk .dialog h3', table)).to.have.property(
        'innerText',
        'Test-Some-Stuff'
      );
      expect($('details-dialog-sk .dialog table.comments tr.comment', table)).to.have.length(0);
      expect($$('details-dialog-sk .dialog h3 a', table)).to.have.property(
        'href',
        'https://some-other-swarming.appspot.com/tasklist?f=sk_name%3ATest-Some-Stuff'
      );
    });

    it('displays commits', async () => {
      const table = await setupWithResponse(incrementalResponse0);
      expect($('[data-commit-index="1"]', table)).to.have.length(1);
      (<HTMLDivElement>$$('[data-commit-index="1"]', table)).click();
      expect($$('details-dialog-sk .dialog h3', table)).to.have.property(
        'innerText',
        '2nd from HEAD'
      );
      expect($('details-dialog-sk .dialog table.comments tr.comment', table)).to.have.length(1);
    });
  });

  /**
   * Extra set of tests that break TS rules to peek at the underlying data.
   */
  describe('internal data', () => {
    const internalData = async (): Promise<any> => {
      const table = (await setupWithResponse(incrementalResponse0)) as any;
      return table.data;
    };

    it('loads tasks correctly', async () => {
      const commitsData = await internalData();
      expect(commitsData.tasks.get('99999')).to.deep.equal({
        commits: ['abc123'],
        name: 'Build-Some-Stuff',
        id: '99999',
        revision: 'abc123',
        status: 'SUCCESS',
        swarmingTaskId: 'swarmy0',
        taskExecutor: '',
      });
      expect(commitsData.tasks.get('11111')).to.deep.equal({
        commits: ['parentofabc123'],
        id: '11111',
        name: 'Test-Some-Stuff',
        revision: 'parentofabc123',
        status: 'FAILURE',
        swarmingTaskId: 'swarmy0',
        taskExecutor: 'some-other-swarming.appspot.com',
      });
      expect(commitsData.tasks.get('77777')).to.deep.equal({
        commits: ['acommitthatisnotlisted'],
        id: '77777',
        name: 'Upload-Some-Stuff',
        revision: 'acommitthatisnotlisted',
        status: 'SUCCESS',
        swarmingTaskId: 'swarmy0',
        taskExecutor: '',
      });
      expect(commitsData.tasks).to.have.keys('99999', '11111', '77777');
    });

    it('loads ancillary data correctly', async () => {
      const commitsData = await internalData();
      expect(commitsData.branchHeads).to.deep.equal([branch0, branch1]);
    });

    it('extracts reverts and relands correctly', async () => {
      const commitsData = await internalData();
      expect(commitsData.revertedMap.get('bad')).to.include({
        hash: '1revertbad',
      });
      expect(commitsData.relandedMap.get('bad')).to.include({
        hash: 'relandbad',
      });
    });

    it('extracts categories', async () => {
      const commitsData = await internalData();
      // Category 'Upload' is not included since no listed commits reference it.
      expect(commitsData.categories).to.have.keys('Build', 'Test');
    });

    it('loads tasks by commit', async () => {
      const commitsData = await internalData();
      expect(commitsData.tasksByCommit).to.have.keys(
        'abc123',
        'parentofabc123',
        'acommitthatisnotlisted'
      );
      expect(commitsData.tasksByCommit.get('abc123')).to.have.keys('Build-Some-Stuff');
      // Task by Commit/TaskSpec reference same underlying object as task by id.
      expect(commitsData.tasksByCommit.get('abc123')!.get('Build-Some-Stuff')).equal(
        commitsData.tasks.get('99999')
      );
    });

    it('loads comments', async () => {
      const commitsData = await internalData();
      // Category 'Upload' is not included since no listed commits reference it.
      expect(commitsData.comments).to.have.keys(commentCommit.commit, commentTask.commit, '');
      // TaskSpec comment.
      expect(commitsData.comments.get('')).to.have.keys(commentTaskSpec.taskSpecName);
      expect(commitsData.comments.get('')!.get(commentTaskSpec.taskSpecName)![0]).to.deep.include({
        message: commentTaskSpec.message,
      });
      // Commit comment.
      expect(commitsData.comments.get(commentCommit.commit)).to.have.keys('');
      expect(commitsData.comments.get(commentCommit.commit)!.get('')![0]).to.deep.include({
        message: commentCommit.message,
      });
      // Task comment.
      expect(commitsData.comments.get(commentTask.commit)).to.have.keys(commentTask.taskSpecName);
      expect(
        commitsData.comments.get(commentTask.commit)!.get(commentTask.taskSpecName)![0]
      ).to.deep.include({ message: commentTask.message });
    });
  });
});
