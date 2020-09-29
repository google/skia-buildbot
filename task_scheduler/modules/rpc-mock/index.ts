import {
  AddSkipTaskRuleRequest,
  AddSkipTaskRuleResponse,
  CancelJobRequest,
  CancelJobResponse,
  DeleteSkipTaskRuleRequest,
  DeleteSkipTaskRuleResponse,
  GetJobRequest,
  GetJobResponse,
  GetTaskRequest,
  GetTaskResponse,
  GetSkipTaskRulesRequest,
  GetSkipTaskRulesResponse,
  Job,
  MockRPCsForTesting,
  SearchJobsRequest,
  SearchJobsResponse,
  SearchTasksRequest,
  SearchTasksResponse,
  Task,
  TaskSchedulerService,
  TriggerJobsRequest,
  TriggerJobsResponse,
} from '../rpc';

export * from './fake-data';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks() {
  MockRPCsForTesting(new FakeTaskSchedulerService())
}

/**
 * FakeTaskSchedulerService provides a mocked implementation of
 * TaskSchedulerService.
 */
class FakeTaskSchedulerService implements TaskSchedulerService {
  private jobs: {[key:string]:Job} = {};
  private tasks: {[key:string]:Task} = {};
  private jobID: number = 0;

  triggerJobs(triggerJobsRequest: TriggerJobsRequest): Promise<TriggerJobsResponse> {
    const ids = triggerJobsRequest.jobs!.map((job) => "" + this.jobID++);
    return Promise.resolve({
      jobIds: ids,
    })
  }
  getJob(getJobRequest: GetJobRequest): Promise<GetJobResponse> {
    return Promise.resolve({
      job: this.jobs[getJobRequest.id],
    });
  }
  cancelJob(cancelJobRequest: CancelJobRequest): Promise<CancelJobResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  searchJobs(searchJobsRequest: SearchJobsRequest): Promise<SearchJobsResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  getTask(getTaskRequest: GetTaskRequest): Promise<GetTaskResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  searchTasks(searchTasksRequest: SearchTasksRequest): Promise<SearchTasksResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  getSkipTaskRules(getSkipTaskRulesRequest: GetSkipTaskRulesRequest): Promise<GetSkipTaskRulesResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  addSkipTaskRule(addSkipTaskRuleRequest: AddSkipTaskRuleRequest): Promise<AddSkipTaskRuleResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
  deleteSkipTaskRule(deleteSkipTaskRuleRequest: DeleteSkipTaskRuleRequest): Promise<DeleteSkipTaskRuleResponse> {
    return new Promise((_, reject) => { reject("not implemented")});
  }
}