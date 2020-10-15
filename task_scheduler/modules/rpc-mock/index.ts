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
import { job1, task0, task1, task2, task3, task4, job2 } from './fake-data';
import { JobStatus } from '../rpc/rpc';

export * from './fake-data';

/**
 * FakeTaskSchedulerService provides a mocked implementation of
 * TaskSchedulerService.
 */
export class FakeTaskSchedulerService implements TaskSchedulerService {
  private jobs: { [key: string]: Job } = {
    [job1.id]: job1,
    [job2.id]: job2,
  };
  private tasks: { [key: string]: Task } = {
    [task0.id]: task0,
    [task1.id]: task1,
    [task2.id]: task2,
    [task3.id]: task3,
    [task4.id]: task4,
  };
  private jobID: number = 0;

  triggerJobs(
    triggerJobsRequest: TriggerJobsRequest
  ): Promise<TriggerJobsResponse> {
    const ids = triggerJobsRequest.jobs!.map((job) => '' + this.jobID++);
    return Promise.resolve({
      jobIds: ids,
    });
  }
  getJob(getJobRequest: GetJobRequest): Promise<GetJobResponse> {
    return Promise.resolve({
      job: this.jobs[getJobRequest.id],
    });
  }
  cancelJob(cancelJobRequest: CancelJobRequest): Promise<CancelJobResponse> {
    const job = this.jobs[cancelJobRequest.id];
    job.status = JobStatus.JOB_STATUS_CANCELED;
    return Promise.resolve({
      job: job,
    });
  }
  searchJobs(
    searchJobsRequest: SearchJobsRequest
  ): Promise<SearchJobsResponse> {
    console.log(searchJobsRequest);
    const results: Job[] = Object.values(this.jobs).filter((job: Job) => {
      const rs = searchJobsRequest.repoState;
      if (rs) {
        if (rs.repo != '' && job.repoState?.repo != rs.repo) {
          console.log('repo');
          return false;
        } else if (
          rs.revision != '' &&
          job.repoState?.revision != rs.revision
        ) {
          console.log('revision');
          return false;
        } else if (rs.patch) {
          if (
            rs.patch.issue != '' &&
            rs.patch.issue != job.repoState!.patch!.issue
          ) {
            console.log('issue');
            return false;
          } else if (
            rs.patch.patchset != '' &&
            rs.patch.patchset != job.repoState?.patch?.patchset
          ) {
            console.log('patchset');
            return false;
          }
        }
      }
      if (searchJobsRequest.name != '' && job.name != searchJobsRequest.name) {
        console.log('name');
        return false;
      }
      if (
        searchJobsRequest.buildbucketBuildId != 0 &&
        job.buildbucketBuildId != searchJobsRequest.buildbucketBuildId
      ) {
        console.log('buildbucketBuildId');
        return false;
      }
      // TODO: Won't match if unset.
      if (
        (searchJobsRequest.status as string) != '' &&
        job.status != searchJobsRequest.status
      ) {
        console.log('status');
        return false;
      }
      if (
        searchJobsRequest.timeStart &&
        new Date(job.createdAt!).getTime() <=
          new Date(searchJobsRequest.timeStart).getTime()
      ) {
        console.log('timeStart');
        return false;
      }
      if (
        searchJobsRequest.timeEnd &&
        new Date(job.createdAt!).getTime() >
          new Date(searchJobsRequest.timeEnd).getTime()
      ) {
        console.log('timeEnd');
        return false;
      }
      // TODO
      if (
        searchJobsRequest.isForce &&
        job.isForce != searchJobsRequest.isForce
      ) {
        console.log('isForce');
        return false;
      }
      return true;
    });
    console.log(results);
    return Promise.resolve({
      jobs: results,
    });
  }
  getTask(getTaskRequest: GetTaskRequest): Promise<GetTaskResponse> {
    return Promise.resolve({
      task: this.tasks[getTaskRequest.id],
    });
  }
  searchTasks(
    searchTasksRequest: SearchTasksRequest
  ): Promise<SearchTasksResponse> {
    return new Promise((_, reject) => {
      reject('not implemented');
    });
  }
  getSkipTaskRules(
    getSkipTaskRulesRequest: GetSkipTaskRulesRequest
  ): Promise<GetSkipTaskRulesResponse> {
    return new Promise((_, reject) => {
      reject('not implemented');
    });
  }
  addSkipTaskRule(
    addSkipTaskRuleRequest: AddSkipTaskRuleRequest
  ): Promise<AddSkipTaskRuleResponse> {
    return new Promise((_, reject) => {
      reject('not implemented');
    });
  }
  deleteSkipTaskRule(
    deleteSkipTaskRuleRequest: DeleteSkipTaskRuleRequest
  ): Promise<DeleteSkipTaskRuleResponse> {
    return new Promise((_, reject) => {
      reject('not implemented');
    });
  }
}
