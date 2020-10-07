import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import {
  incrementalResponse0,
  responseSingleCommitTask,
  responseMultiCommitTask,
  responseNoncontiguousCommitsTask,
  responseTasksToFilter,
  branch0,
  branch1,
  commentCommit,
  commentTask,
  commentTaskSpec,
} from '../rpc-mock/test_data';
import { GetIncrementalCommitsResponse } from '../rpc';
import { expect } from 'chai';
import { CommitsTableSk } from './commits-table-sk';
import { SetupMocks } from '../rpc-mock';

describe('commits-table-sk', () => {
  const newTableInstance = setUpElementUnderTest('commits-table-sk');

  beforeEach(async () => {});

  let setupWithResponse = async (resp: GetIncrementalCommitsResponse) => {
    SetupMocks().expectGetIncrementalCommits(resp);
    const ep = eventPromise('end-task');
    const table = newTableInstance((el) => ((<CommitsTableSk>el).filter = 'All')) as CommitsTableSk;
    await ep;
    return table;
  };

  it('displays multiple commit tasks', async () => {
    const table = await setupWithResponse(responseMultiCommitTask);
    expect($('.task', table)).to.have.length(1);
    expect($$('.task', table)!.classList.value).to.include('task-failure');
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

  it('filters task specs', async () => {
    const table = await setupWithResponse(responseTasksToFilter);
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Green-Spec',
      'Always-Red-Spec',
      'Interesting-Spec',
      'Only-Failed-On-Commented-Commit-Spec',
    ]);

    const clickLabel = (i: number, expectText: string) => {
      const label = $('label.specFilter', table)[i] as HTMLLabelElement;
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
    ]);

    clickLabel(2, 'Comments');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Red-Spec',
    ]);
    clickLabel(3, 'Failing w/o comment');
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Interesting-Spec',
    ]);

    const searchbox = $$('input-sk input', table) as HTMLInputElement;
    searchbox.value = 'Always';
    const ep = eventPromise('change');
    searchbox.dispatchEvent(new Event('change', { bubbles: true }));
    await ep;
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Green-Spec',
      'Always-Red-Spec',
    ]);
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
        swarmingTaskId: 'swarmy',
      });
      expect(commitsData.tasks.get('11111')).to.deep.equal({
        commits: ['parentofabc123'],
        id: '11111',
        name: 'Test-Some-Stuff',
        revision: 'parentofabc123',
        status: 'FAILURE',
        swarmingTaskId: 'swarmy',
      });
      expect(commitsData.tasks.get('77777')).to.deep.equal({
        commits: ['acommitthatisnotlisted'],
        id: '77777',
        name: 'Upload-Some-Stuff',
        revision: 'acommitthatisnotlisted',
        status: 'SUCCESS',
        swarmingTaskId: 'swarmy',
      });
      expect(commitsData.tasks).to.have.keys('99999', '11111', '77777');
    });

    it('loads ancillary data correctly', async () => {
      const commitsData = await internalData();
      expect(commitsData.branchHeads).to.deep.equal([branch0, branch1]);
    });

    it('extracts reverts and relands correctly', async () => {
      const commitsData = await internalData();
      expect(commitsData.revertedMap.get('bad')).to.include({ hash: '1revertbad' });
      expect(commitsData.relandedMap.get('bad')).to.include({ hash: 'relandbad' });
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
