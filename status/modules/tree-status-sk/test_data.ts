import { RoleResp, TreeStatusResp } from './tree-status-sk';

export const treeStatusResp: TreeStatusResp = {
  message: 'No longer Broken!',
  username: 'alice@google.com',
  date: '2020-10-08 22:51:02.575754',
  general_state: 'open',
};

export const generalRoleResp: RoleResp = { username: 'alice@google.com' };
export const gpuRoleResp: RoleResp = { username: 'bob@google.com' };
export const androidRoleResp: RoleResp = { username: 'christy@google.com' };
export const infraRoleResp: RoleResp = { username: 'dan@google.com' };
