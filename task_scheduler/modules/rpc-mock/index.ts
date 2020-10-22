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
import {
  job1,
  task0,
  task1,
  task2,
  task3,
  task4,
  job2,
  skipRule1,
  skipRule2,
  skipRule3,
} from './fake-data';
import { JobStatus, SkipTaskRule } from '../rpc/rpc';

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
  private skipRules = [skipRule1, skipRule2, skipRule3];

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
  searchJobs(req: SearchJobsRequest): Promise<SearchJobsResponse> {
    console.log(req);
    const results: Job[] = Object.values(this.jobs).filter((job: Job) => {
      if (req.hasRepo && job.repoState?.repo != req.repo) {
        return false;
      } else if (req.hasRevision && job.repoState?.revision != req.revision) {
        return false;
      } else if (req.hasIssue && req.issue != job.repoState!.patch!.issue) {
        return false;
      } else if (
        req.hasPatchset &&
        req.patchset != job.repoState?.patch?.patchset
      ) {
        return false;
      } else if (req.hasName && job.name != req.name) {
        return false;
      } else if (
        req.hasBuildbucketBuildId &&
        job.buildbucketBuildId != req.buildbucketBuildId
      ) {
        return false;
      } else if (req.hasStatus && job.status != req.status) {
        return false;
      } else if (
        req.hasTimeStart &&
        new Date(job.createdAt!).getTime() <= new Date(req.timeStart!).getTime()
      ) {
        return false;
      } else if (
        req.hasTimeEnd &&
        new Date(job.createdAt!).getTime() > new Date(req.timeEnd!).getTime()
      ) {
        return false;
      } else if (req.hasIsForce && job.isForce != req.isForce) {
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
    return Promise.resolve({ rules: this.skipRules.slice() });
  }
  addSkipTaskRule(
    addSkipTaskRuleRequest: AddSkipTaskRuleRequest
  ): Promise<AddSkipTaskRuleResponse> {
    this.skipRules.push({
      addedBy: 'you@google.com',
      taskSpecPatterns: addSkipTaskRuleRequest.taskSpecPatterns,
      commits: addSkipTaskRuleRequest.commits,
      description: addSkipTaskRuleRequest.description,
      name: addSkipTaskRuleRequest.name,
    });
    return Promise.resolve({ rules: this.skipRules.slice() });
  }
  deleteSkipTaskRule(
    deleteSkipTaskRuleRequest: DeleteSkipTaskRuleRequest
  ): Promise<DeleteSkipTaskRuleResponse> {
    this.skipRules = this.skipRules.filter(
      (rule: SkipTaskRule) => rule.name != deleteSkipTaskRuleRequest.id
    );
    return Promise.resolve({ rules: this.skipRules.slice() });
  }
}
