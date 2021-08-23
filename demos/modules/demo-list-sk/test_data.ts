import { Metadata } from '../rpc_types';

export const singleDemoEntry: Metadata = {
  revision: {
    url: 'example.com',
    hash: '123',
  },
  demos: ['demo0'],
};

export const twoDemoEntries: Metadata = {
  revision: {
    url: 'example.com',
    hash: '123',
  },
  demos: ['demo0', 'demo1'],
};
