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
  SearchJobsRequest,
  SearchJobsResponse,
  SearchTasksRequest,
  SearchTasksResponse,
  Task,
  TaskSchedulerService,
  TriggerJobsRequest,
  TriggerJobsResponse,
} from '../rpc';
import { job1, task0, task1, task2, task3, task4 } from './fake-data';

export * from './fake-data';

/**
 * FakeTaskSchedulerService provides a mocked implementation of
 * TaskSchedulerService.
 */
export class FakeTaskSchedulerService implements TaskSchedulerService {
  private jobs: {[key:string]:Job} = {
    [job1.id]: job1,
  };
  private tasks: {[key:string]:Task} = {
    [task0.id]: task0,
    [task1.id]: task1,
    [task2.id]: task2,
    [task3.id]: task3,
    [task4.id]: task4,
  };
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
    return Promise.resolve({
      task: this.tasks[getTaskRequest.id],
    });
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