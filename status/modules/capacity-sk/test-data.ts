import { GetBotUsageResponse } from '../rpc/status';

export const resp: GetBotUsageResponse = {
  botSets: [
    {
      dimensions: {
        gpu: '1002:6613', os: 'foster', pool: 'Skia', purple: 'elephant',
      },
      botCount: 5,
      totalTasks: 10,
      msPerCommit: 60 * 60 * 1000,
      cqTasks: 3,
      msPerCq: 20 * 60 * 1000,
    },
    {
      dimensions: { os: 'windows95', pool: 'Skia' },
      botCount: 1,
      totalTasks: 10,
      msPerCommit: 60 * 60 * 1000,
      cqTasks: 5,
      msPerCq: 30 * 60 * 1000,
    },
    {
      dimensions: {
        gpu: 'widget5', os: 'Android', pool: 'Skia', device: 'marlin',
      },
      botCount: 25,
      totalTasks: 8,
      msPerCommit: 6 * 60 * 60 * 1000,
      cqTasks: 7,
      msPerCq: 5 * 60 * 60 * 1000,
    },
    {
      dimensions: { cpu: 'intel', os: 'foster', pool: 'Skia' },
      botCount: 5,
      totalTasks: 15,
      msPerCommit: 3 * 60 * 60 * 1000,
      cqTasks: 3,
      msPerCq: 20 * 60 * 1000,
    },
  ],
};
