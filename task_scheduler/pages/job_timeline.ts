import '../modules/job-timeline-sk';
import '../modules/task-scheduler-scaffold-sk';
import '../modules/colors.css';

import {
  GetTaskSchedulerService,
  GetJobResponse,
  TaskSummaries,
  TaskSummary,
  GetTaskResponse,
} from '../modules/rpc';
import { JobTimelineSk } from '../modules/job-timeline-sk/job-timeline-sk';

const ele = <JobTimelineSk>document.querySelector('job-timeline-sk');
const rpc = GetTaskSchedulerService(ele);
rpc
  .getJob({
    id: '{{.JobId}}',
  })
  .then((jobResp: GetJobResponse) => {
    const taskIds = jobResp
      .job!.tasks!.map((tasks: TaskSummaries) => tasks.tasks!)
      .reduce((acc: TaskSummary[], subArray: TaskSummary[]) =>
        acc.concat(subArray)
      )
      .map((task: TaskSummary) => task.id);
    Promise.all(
      taskIds.map((id: string) =>
        rpc.getTask({
          id: id,
          includeStats: true,
        })
      )
    ).then((taskResps: GetTaskResponse[]) => {
      ele.draw(
        jobResp.job!,
        taskResps.map((resp: GetTaskResponse) => resp.task!),
        []
      );
    });
  });
