import { commentTaskSpec } from '../rpc-mock';
import { Comment } from '../rpc';

export const taskspecComments = (() => {
  const ret: Array<Comment> = [];
  ret.push(JSON.parse(JSON.stringify(commentTaskSpec)) as Comment);
  ret[0].message = 'Flakiness caused by the widget.';
  ret[0].user = 'alice@google.com';
  ret[0].timestamp = new Date('9/22/2020, 10:21:52 AM UTC').toISOString();
  ret[0].flaky = true;
  ret.push(JSON.parse(JSON.stringify(commentTaskSpec)) as Comment);
  ret[1].message = "Rewrite of widget's thingamabob in cl/123.";
  ret[1].user = 'bob@google.com';
  ret[1].timestamp = new Date('9/23/2020, 02:21:52 PM UTC').toISOString();
  ret[1].ignoreFailure = true;
  return ret;
})();
