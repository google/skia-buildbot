import { RoleResp, TreeStatusResp } from './tree-status-sk';

export const treeStatusResp: TreeStatusResp = {
  message: 'No longer Broken!',
  username: 'alice@google.com',
  date: '2020-10-08 22:51:02.575754',
  general_state: 'open',
};

export const treeStatusResp2: TreeStatusResp = {
  message: 'Closed: Broken again!',
  username: 'superman@google.com',
  date: '2020-10-08 22:58:02.575754',
  general_state: 'closed',
};

export const treeStatusResp3: TreeStatusResp = {
  message: 'Open: I fixed it (again)!',
  username: 'batman@google.com',
  date: '2020-10-08 23:51:02.575754',
  general_state: 'open',
};

export const generalRoleResp: RoleResp = { emails: ['alice@google.com'] };
export const gpuRoleResp: RoleResp = { emails: ['bob@google.com'] };
export const androidRoleResp: RoleResp = { emails: ['christy@google.com'] };
export const infraRoleResp: RoleResp = { emails: ['dan@google.com'] };
