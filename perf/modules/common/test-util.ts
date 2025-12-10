// Functions to be used by test and demo files. These helps create dummy data and API mocks.
import fetchMock from 'fetch-mock';
import { Status } from '../../../infra-sk/modules/json';
import { QueryConfig } from '../json';

const getCookieValue = (name: string) =>
  document.cookie.match('(^|;)\\s*' + name + '\\s*=\\s*([^;]+)')?.pop() || '';

export function setUpExploreDemoEnv() {
  // The demo server will inject this cookie if there is a backend.
  if (getCookieValue('proxy_endpoint')) {
    return;
  }
  const status: Status = {
    email: 'user@google.com',
    roles: ['viewer', 'admin', 'editor', 'bisecter'],
  };

  fetchMock.get('/_/login/status', status);

  const paramSet = {
    arch: ['arm', 'arm64', 'x86_64'],
    bench_type: ['skandroidcodec'],
    compiler: ['Clang'],
    config: ['nonrendering'],
    configuration: ['OptimizeForSize'],
    cpu_or_gpu: ['CPU'],
    cpu_or_gpu_value: ['AVX2', 'AVX512', 'Snapdragon855', 'SnapdragonQM215'],
    extra_config: [
      'Android',
      'Android_Wuffs',
      'ColorSpaces',
      'Fast',
      'SK_FORCE_RASTER_PIPELINE_BLITTER',
      'Wuffs',
    ],
    model: ['GCE', 'JioNext', 'MacBookPro11.5', 'NUC9i7QN', 'Pixel4'],
    name: ['AndroidCodec_01_original.jpg_SampleSize2'],
    os: ['Android', 'Debian10', 'Debian11', 'Mac10.13', 'Win2019'],
    source_type: ['image'],
    sub_result: ['min_ms', 'min_ratio'],
  };

  fetchMock.get(/_\/initpage\/.*/, () => ({
    dataframe: {
      traceset: null,
      header: null,
      paramset: paramSet,
      skip: 0,
    },
    ticks: [],
    skps: [],
    msg: '',
  }));

  fetchMock.post('/_/count/', {
    count: 117, // Don't make the demo page non-deterministic.
    paramset: paramSet,
  });

  fetchMock.post('/_/frame/start', {
    status: 'Running',
    messages: [],
    url: '/_/status/d25fedcc-7e36-47e4-83d5-58ab76b2d3d1',
  });

  const defaultConfig: QueryConfig = {
    default_param_selections: null,
    default_url_values: null,
    include_params: ['arch', 'config', 'bench_type', 'compiler', 'model', 'os', 'sub_result'],
  };

  fetchMock.get('/_/defaults/', defaultConfig);

  const normalTracesResponse = {
    status: 'Finished',
    messages: [
      {
        key: 'Step',
        value: '1/1',
      },
    ],
    results: {
      dataframe: {
        traceset: {
          ',arch=arm,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=SnapdragonQM215,extra_config=Android,model=JioNext,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ms,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,':
            [
              61.2075, 60.687603, 61.30078, 61.660313, 60.830208, 60.854946, 60.8525, 61.43297,
              61.24557, 61.098125, 61.284843, 60.7938, 61.741615, 62.60328, 60.93729, 60.925156,
              63.232346, 61.770676, 62.252968, 61.87958, 61.140102, 62.40708, 62.869167, 60.893852,
              61.042187, 61.17974, 61.73057, 61.754063, 60.726772, 61.837135, 61.868282, 61.161095,
              61.88469, 60.81271, 61.4625, 60.91443, 60.806095, 60.81344, 61.624477, 60.98828,
              60.838856, 61.989845, 60.84349, 61.973698, 61.97073, 60.615208, 62.083595, 61.148228,
              1e32, 1e32,
            ],
          ',arch=arm,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=SnapdragonQM215,extra_config=Android,model=JioNext,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ratio,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,':
            [
              1.0053873, 1.0019164, 1.0029848, 1.002643, 1.0012664, 1.0028929, 1.0037411, 1.003003,
              1.0050658, 1.007233, 1.0022581, 1.001872, 1.0027552, 1.0019709, 1.0021086, 1.0030998,
              1.0080754, 1.0013162, 1.0025258, 1.0044054, 1.0017514, 1.0026898, 1.0032914,
              1.0032947, 1.0027568, 1.0056816, 1.0076947, 1.0022088, 1.0029486, 1.0037018,
              1.0043061, 1.0032768, 1.0015746, 1.0046197, 1.0041125, 1.0060117, 1.0032651,
              1.0015031, 1.0050989, 1.0046166, 1.0052769, 1.0035608, 1.0040259, 1.002186, 1.0046998,
              1.0016583, 1.0048993, 1.0062689, 1e32, 1e32,
            ],
          ',arch=arm64,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=Snapdragon855,extra_config=Android_Wuffs,model=Pixel4,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ms,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,':
            [
              13.51672, 13.434168, 13.47146, 13.494012, 13.464116, 13.450209, 13.423439, 13.434011,
              13.502189, 13.435418, 13.398959, 13.401876, 13.412658, 13.531095, 13.503022,
              13.520366, 13.41073, 13.391043, 13.389377, 13.370262, 13.394116, 13.366512, 13.373126,
              13.494376, 13.482189, 13.390887, 13.423231, 13.388387, 13.369949, 13.377084,
              13.387605, 13.409533, 13.423283, 13.372189, 13.372918, 13.435366, 13.38495, 13.405939,
              13.390105, 13.502606, 13.381928, 13.329532, 13.420783, 13.419793, 13.440002, 1e32,
              13.488907, 1e32, 13.49323, 1e32,
            ],
          ',arch=arm64,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=Snapdragon855,extra_config=Android_Wuffs,model=Pixel4,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ratio,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,':
            [
              1.0060265, 1.0050516, 1.0032824, 1.0048633, 1.0030173, 1.0089877, 1.005564, 1.0143331,
              1.0087292, 1.004551, 1.0055237, 1.0070225, 1.0050403, 1.0034873, 1.0055274, 1.0046265,
              1.0032196, 1.0046984, 1.0029291, 1.0070547, 1.0091536, 1.007189, 1.0019591, 1.0073526,
              1.0068724, 1.0070788, 1.0009079, 1.0057614, 1.0018076, 1.0048864, 1.0045946,
              1.0053095, 1.0055135, 1.0083896, 1.007283, 1.009362, 1.0063659, 1.0073311, 1.0077171,
              1.0068235, 1.0078387, 1.010589, 1.0133693, 1.001502, 1.0090255, 1e32, 1.0033516, 1e32,
              1.0042962, 1e32,
            ],
        },
        header: [
          {
            offset: 67125,
            timestamp: 1687855198,
          },
          {
            offset: 67126,
            timestamp: 1687857789,
          },
          {
            offset: 67127,
            timestamp: 1687868015,
          },
          {
            offset: 67128,
            timestamp: 1687868368,
          },
          {
            offset: 67129,
            timestamp: 1687870256,
          },
          {
            offset: 67130,
            timestamp: 1687872763,
          },
          {
            offset: 67131,
            timestamp: 1687877748,
          },
          {
            offset: 67132,
            timestamp: 1687878083,
          },
          {
            offset: 67133,
            timestamp: 1687878588,
          },
          {
            offset: 67134,
            timestamp: 1687878658,
          },
          {
            offset: 67135,
            timestamp: 1687878976,
          },
          {
            offset: 67136,
            timestamp: 1687879230,
          },
          {
            offset: 67137,
            timestamp: 1687881375,
          },
          {
            offset: 67138,
            timestamp: 1687884748,
          },
          {
            offset: 67139,
            timestamp: 1687885047,
          },
          {
            offset: 67140,
            timestamp: 1687885507,
          },
          {
            offset: 67141,
            timestamp: 1687886132,
          },
          {
            offset: 67142,
            timestamp: 1687886787,
          },
          {
            offset: 67143,
            timestamp: 1687887013,
          },
          {
            offset: 67144,
            timestamp: 1687888513,
          },
          {
            offset: 67145,
            timestamp: 1687891891,
          },
          {
            offset: 67146,
            timestamp: 1687891925,
          },
          {
            offset: 67147,
            timestamp: 1687895229,
          },
          {
            offset: 67148,
            timestamp: 1687895693,
          },
          {
            offset: 67149,
            timestamp: 1687896092,
          },
          {
            offset: 67150,
            timestamp: 1687896114,
          },
          {
            offset: 67151,
            timestamp: 1687896459,
          },
          {
            offset: 67152,
            timestamp: 1687900291,
          },
          {
            offset: 67153,
            timestamp: 1687900389,
          },
          {
            offset: 67154,
            timestamp: 1687900992,
          },
          {
            offset: 67155,
            timestamp: 1687904682,
          },
          {
            offset: 67156,
            timestamp: 1687907669,
          },
          {
            offset: 67157,
            timestamp: 1687909158,
          },
          {
            offset: 67158,
            timestamp: 1687910749,
          },
          {
            offset: 67159,
            timestamp: 1687911636,
          },
          {
            offset: 67160,
            timestamp: 1687911698,
          },
          {
            offset: 67161,
            timestamp: 1687913983,
          },
          {
            offset: 67162,
            timestamp: 1687914369,
          },
          {
            offset: 67163,
            timestamp: 1687917173,
          },
          {
            offset: 67164,
            timestamp: 1687927827,
          },
          {
            offset: 67165,
            timestamp: 1687928532,
          },
          {
            offset: 67166,
            timestamp: 1687928754,
          },
          {
            offset: 67167,
            timestamp: 1687930648,
          },
          {
            offset: 67168,
            timestamp: 1687933565,
          },
          {
            offset: 67169,
            timestamp: 1687936673,
          },
          {
            offset: 67170,
            timestamp: 1687958245,
          },
          {
            offset: 67171,
            timestamp: 1687958371,
          },
          {
            offset: 67172,
            timestamp: 1687958912,
          },
          {
            offset: 67173,
            timestamp: 1687960354,
          },
          {
            offset: 67174,
            timestamp: 1687961972,
          },
        ],
        paramset: {
          arch: ['arm', 'arm64', 'x86_64'],
          bench_type: ['skandroidcodec'],
          compiler: ['Clang'],
          config: ['nonrendering'],
          configuration: ['OptimizeForSize'],
          cpu_or_gpu: ['CPU'],
          cpu_or_gpu_value: ['AVX2', 'AVX512', 'Snapdragon855', 'SnapdragonQM215'],
          extra_config: [
            'Android',
            'Android_Wuffs',
            'ColorSpaces',
            'Fast',
            'SK_FORCE_RASTER_PIPELINE_BLITTER',
            'Wuffs',
          ],
          model: ['GCE', 'JioNext', 'MacBookPro11.5', 'NUC9i7QN', 'Pixel4'],
          name: ['AndroidCodec_01_original.jpg_SampleSize2'],
          os: ['Android', 'Debian10', 'Debian11', 'Mac10.13', 'Win2019'],
          source_type: ['image'],
          sub_result: ['min_ms', 'min_ratio'],
          test: ['AndroidCodec_01_original.jpg_SampleSize2_640_480'],
        },
        skip: 0,
      },
      skps: [],
      msg: '',
      display_mode: 'display_plot',
      anomalymap: {
        ',arch=arm,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=SnapdragonQM215,extra_config=Android,model=JioNext,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ms,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,':
          {
            67130: {
              id: '123',
              test_path:
                ',arch=arm,bench_type=skandroidcodec,compiler=Clang,config=nonrendering,cpu_or_gpu=CPU,cpu_or_gpu_value=SnapdragonQM215,extra_config=Android,model=JioNext,name=AndroidCodec_01_original.jpg_SampleSize2,os=Android,source_type=image,sub_result=min_ms,test=AndroidCodec_01_original.jpg_SampleSize2_640_480,',
              bug_id: 0,
              start_revision: 67129,
              end_revision: 67130,
              is_improvement: false,
              recovered: false,
              state: 'untriaged',
              statistic: 'avg',
              units: 'ms',
              degrees_of_freedom: 1,
              median_before_anomaly: 60.830208,
              median_after_anomaly: 60.854946,
              p_value: 0.01,
              segment_size_after: 10,
              segment_size_before: 10,
              std_dev_before_anomaly: 1,
              t_statistic: 5,
              subscription_name: 'test',
              bug_component: '',
              bug_labels: [],
              bug_cc_emails: [],
              bisect_ids: [],
            },
          },
      },
    },
    url: '/_/status/d25fedcc-7e36-47e4-83d5-58ab76b2d3d1',
  };

  fetchMock.get('/_/status/d25fedcc-7e36-47e4-83d5-58ab76b2d3d1', normalTracesResponse);

  fetchMock.post('/_/cid/', {
    commitSlice: [
      {
        offset: 67193,
        hash: '0d7087e5b99087f5945f04dbda7b7a7a4b12e344',
        ts: 1687990261,
        author: 'John Stiles (johnstiles@google.com)',
        message: 'Remove Win10 + ANGLE + IrisXe test and perf jobs.',
        url: 'https://skia.googlesource.com/skia/+show/0d7087e5b99087f5945f04dbda7b7a7a4b12e344',
        body: '',
      },
      {
        offset: 67194,
        hash: '2894e7194406ad8014d3e85b39379ca0e4607ead',
        ts: 1687991201,
        author: 'Arman Uguray (armansito@google.com)',
        message: 'Roll vello from ef2630ad to 12e764d5',
        url: 'https://skia.googlesource.com/skia/+show/2894e7194406ad8014d3e85b39379ca0e4607ead',
        body: '',
      },
    ],
    logEntry:
      'commit 0d7087e5b99087f5945f04dbda7b7a7a4b12e344\nAuthor John Stiles (johnstiles@google.com)\nDate 28 Jun 23 22:11 +0000\n\nRemove Win10 + ANGLE + IrisXe test and perf jobs.\n\nOnce skia:14417 is resolved, we should reinstate these jobs.\n\nBug: skia:14417\nChange-Id: Ib6b2a06cf7983c998d1d4e95a5e4973377b3bd48\nReviewed-on: https://skia-review.googlesource.com/c/skia/+/718157\nAuto-Submit: John Stiles \u003cjohnstiles@google.com\u003e\nCommit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\nCommit-Queue: John Stiles \u003cjohnstiles@google.com\u003e\nReviewed-by: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',
  });

  fetchMock.post('/_/details/?results=false', {
    gitHash: 'e539c1a62d339f6509463a7e59d83141576e3722',
    key: {
      arch: 'arm',
      compiler: 'Clang',
      cpu_or_gpu: 'CPU',
      cpu_or_gpu_value: 'SnapdragonQM215',
      extra_config: 'Android',
      model: 'JioNext',
      os: 'Android',
    },
    swarming_bot_id: 'skia-rpi2-rack1-shelf1-026',
    swarming_task_id: '631c9c79d2c59211',
  });

  fetchMock.post('/_/shortcut/get', {
    GraphConfig: {
      queries: ['arch=arm64&bench_type=skandroidcodec'],
    },
  });

  fetchMock.post('/_/nextParamList/', {
    paramset: paramSet,
    count: 4,
  });

  fetchMock.post('/_/shortcut/update', {
    id: 'aaab78c9711cb79197d47f448ba51338',
  });
}
