import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { incrementalResponse0, responseSingleCommitTask } from '../rpc-mock/test_data'
import { GetIncrementalCommitsResponse } from '../rpc';
import { SetupMocks } from '../rpc-mock';
import { expect } from 'chai';
import { CommitsTableSk } from './commits-table-sk';
import { CommitsDataSk } from '../commits-data-sk/commits-data-sk';

describe('commits-table-sk', () => {
  // We use a data instance and it's test data so we don't have to maintain additional, restructed test data.
  const newDataInstance = setUpElementUnderTest('commits-data-sk');
  const newTableInstance = setUpElementUnderTest('commits-table-sk');

  beforeEach(async () => {


  });

  let setupWithResponse = async (resp: GetIncrementalCommitsResponse) => {
    SetupMocks(resp);
    const ep = eventPromise('end-task');
    newDataInstance() as CommitsDataSk;
    await ep;
    return newTableInstance() as CommitsTableSk;
  }

  it('displays contiguous tasks', async () => {
    debugger;
    const table = await setupWithResponse(responseSingleCommitTask);
    expect($$('.task', table)).to.have.length(1);
  });

  it('displays noncontiguous tasks', async () => {
    //
  });

  it('displays commits', async () => {
    //
  });

  it('displays icons', async () => {
    //
  });

  it('aligns tasks with their commits', async () => {
    //
  });

  it('aligns tasks with their taskspecs', async () => {
    //
  });

  it('highlights reverts/relands', async () => {
    //
  });
});
