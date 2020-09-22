import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import {
  incrementalResponse0,
  responseSingleCommitTask,
  responseMultiCommitTask,
  responseNoncontiguousCommitsTask,
  responseTasksToFilter,
} from '../rpc-mock/test_data';
import { GetIncrementalCommitsResponse } from '../rpc';
import { expect } from 'chai';
import { CommitsTableSk } from './commits-table-sk';
import { CommitsDataSk } from '../commits-data-sk/commits-data-sk';
import { SetupMocks } from '../rpc-mock';

describe('commits-table-sk', () => {
  // We use a data instance and it's test data so we don't have to maintain additional,
  // restructed test data.
  const newDataInstance = setUpElementUnderTest('commits-data-sk');
  const newTableInstance = setUpElementUnderTest('commits-table-sk');

  beforeEach(async () => {});

  let setupWithResponse = async (resp: GetIncrementalCommitsResponse) => {
    SetupMocks(resp);
    const ep = eventPromise('end-task');
    newDataInstance() as CommitsDataSk;
    await ep;
    return newTableInstance((el) => ((<CommitsTableSk>el).filter = 'all')) as CommitsTableSk;
  };

  it('displays single commit tasks', async () => {
    const table = await setupWithResponse(responseSingleCommitTask);
    expect($('.task', table)).to.have.length(1);
    expect($$('.task', table)!.classList.value).to.include('task-success');
  });

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

    table.filter = 'interesting';
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Interesting-Spec',
    ]);

    table.filter = 'failures';
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Red-Spec',
      'Interesting-Spec',
    ]);

    table.filter = 'nocomment';
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Interesting-Spec',
    ]);

    table.filter = 'comments';
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Red-Spec',
    ]);

    table.filter = 'search';
    table.search = 'Always';
    expect($('.task-spec', table).map((el) => el.getAttribute('title'))).to.have.deep.members([
      'Always-Green-Spec',
      'Always-Red-Spec',
    ]);
  });
});
