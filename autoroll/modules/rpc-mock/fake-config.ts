import {
  Config,
  CommitMsgConfig_BuiltIn,
  GerritConfig_Config,
  NotifierConfig_LogLevel,
  NotifierConfig_MsgType,
} from '../config';

export function GetFakeConfig(): Config {
  return {
    rollerName: 'skia-skiabot-test-autoroll',
    childDisplayName: 'Skia',
    childBugLink: 'https://child-bug',
    parentDisplayName: 'Skiabot Test',
    parentBugLink: 'https://parent-bug',
    parentWaterfall: 'https://status-staging.skia.org/repo/skiabot-test',
    ownerPrimary: 'borenet',
    ownerSecondary: 'rmistry',
    contacts: ['borenet@google.com'],
    serviceAccount: 'skia-autoroll@skia-public.iam.gserviceaccount.com',
    useWorkloadIdentity: false,
    isInternal: false,
    reviewer: ['borenet@google.com'],
    rollCooldown: '',
    timeWindow: '',
    supportsManualRolls: true,
    commitMsg: {
      bugProject: '',
      childLogUrlTmpl: 'https://skia.googlesource.com/skia.git/+log/{{.RollingFrom}}..{{.RollingTo}}',
      cqDoNotCancelTrybots: false,
      cqExtraTrybots: undefined,
      includeLog: true,
      includeRevisionCount: true,
      includeTbrLine: true,
      includeTests: true,
      builtIn: CommitMsgConfig_BuiltIn.DEFAULT,
      custom: '',
    },
    gerrit: {
      url: 'https://skia-review.googlesource.com',
      project: 'skiabot-test',
      config: GerritConfig_Config.CHROMIUM,
    },
    kubernetes: {
      cpu: '1',
      memory: '2Gi',
      readinessFailureThreshold: 10,
      readinessInitialDelaySeconds: 30,
      readinessPeriodSeconds: 30,
      disk: '',
      image: 'gcr.io/fake-image',
    },
    parentChildRepoManager: {
      gitilesParent: {
        gitiles: {
          branch: 'master',
          repoUrl: 'https://skia.googlesource.com/skiabot-test.git',
          defaultBugProject: 'skia',
        },
        dep: {
          primary: {
            id: 'https://skia.googlesource.com/skia.git',
            path: 'DEPS',
          },
        },
        gerrit: {
          url: 'https://skia-review.googlesource.com',
          project: 'skiabot-test',
          config: GerritConfig_Config.CHROMIUM,
        },
      },
      gitilesChild: {
        gitiles: {
          branch: 'master',
          repoUrl: 'https://skia.googlesource.com/skia.git',
          defaultBugProject: 'skia',
        },
        path: '',
      },
    },
    notifiers: [{
      msgType: [NotifierConfig_MsgType.LAST_N_FAILED],
      monorail: {
        project: 'skia',
        owner: 'borenet',
        cc: ['rmistry@google.com'],
        components: ['AutoRoll'],
      },
      subject: '',
      logLevel: NotifierConfig_LogLevel.SILENT,
    }],
  };
}
