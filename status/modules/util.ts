// Utility functions and interfaces used by multiple status elements.
import { LongCommit } from './rpc/status';

// Commit with added metadata we compute that aid in displaying and associating it with other data.
export interface Commit extends LongCommit {
  shortAuthor: string;
  shortHash: string;
  shortSubject: string;
  issue: string;
  patchStorage: string;
  isRevert: boolean;
  isReland: boolean;
  ignoreFailure: boolean;
}
