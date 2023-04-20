/**
 * @module modules/cid
 *
 * Functions for working with CommitNumer's, aka Commit IDs.
 */

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { CommitNumber, CIDHandlerResponse } from '../json';

/**
 * Look up the commit ids for the given offsets and sources.
 *
 */
export const lookupCids = (cids: CommitNumber[]): Promise<CIDHandlerResponse> =>
  fetch('/_/cid/', {
    method: 'POST',
    body: JSON.stringify(cids),
    headers: {
      'Content-Type': 'application/json',
    },
  }).then(jsonOrThrow);
