import './index';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitsDataSk } from './commits-data-sk';
import {
  branch0,
  branch1,
  commentCommit,
  commentTask,
  commentTaskSpec,
  incrementalResponse0,
} from '../rpc-mock/test_data';
import { expect } from 'chai';
import { SetupMocks } from '../rpc-mock';

describe('commits-data-sk', () => {
  const newInstance = setUpElementUnderTest('commits-data-sk');
  let commitsData: CommitsDataSk;

  beforeEach(async () => {
    SetupMocks().expectGetIncrementalCommits(incrementalResponse0);
    const ep = eventPromise('end-task');
    commitsData = newInstance() as CommitsDataSk;
    await ep;
  });

  it('loads tasks correctly', async () => {
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
    expect(commitsData.branchHeads).to.deep.equal([branch0, branch1]);
    expect(commitsData.swarmingUrl).to.equal('https://example.com/swarming');
    expect(commitsData.taskSchedulerUrl).to.equal('https://example.com/ts');
  });

  it('extracts reverts and relands correctly', async () => {
    expect(commitsData.revertedMap.get('bad')).to.include({ hash: '1revertbad' });
    expect(commitsData.relandedMap.get('bad')).to.include({ hash: 'relandbad' });
  });

  it('extracts categories', async () => {
    // Category 'Upload' is not included since no listed commits reference it.
    expect(commitsData.categories).to.have.keys('Build', 'Test');
  });

  it('loads tasks by commit', async () => {
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
