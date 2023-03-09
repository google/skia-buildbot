import { GetScreenshotsRPCResponse } from '../rpc_types';

export const rpcResponse: GetScreenshotsRPCResponse = {
  screenshots_by_application: {
    'my-app': [
      {
        test_name: 'alpha',
        url: '/screenshots/my-app_alpha.png',
      },
      {
        test_name: 'beta',
        url: '/screenshots/my-app_beta.png',
      },
      {
        test_name: 'gamma',
        url: '/screenshots/my-app_gamma.png',
      },
    ],
    'another-app': [
      {
        test_name: 'delta',
        url: '/screenshots/another-app_delta.png',
      },
      {
        test_name: 'epsilon',
        url: '/screenshots/another-app_epsilon.png',
      },
    ],
  },
};
