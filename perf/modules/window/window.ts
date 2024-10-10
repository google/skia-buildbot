// Declaration for the window.sk global variable.
import { SkPerfConfig } from '../json';

export function getBuildTag(tag: string = window.perf?.image_tag ?? ''): {
  tag: string | null;
  type: 'invalid' | 'git' | 'louhi' | 'tag';
} {
  const ats = tag.split('@');
  if (ats.length < 2) {
    return { type: 'invalid', tag: null };
  }

  const rawTag = ats[1];
  // Not a tag, return empty.
  if (!ats[1].startsWith('tag:')) {
    return { type: 'invalid', tag: null };
  }

  const gitTag = 'tag:git-';
  if (rawTag.startsWith(gitTag)) {
    // The git hash tag: tag:git-hash, return the first 7 encodings.
    return { type: 'git', tag: rawTag.substring(gitTag.length, gitTag.length + 7) };
  } else if (rawTag.length >= 38 && rawTag.substring(25, 30) === 'louhi') {
    // The louhi build: tag:TIMESTAMP-louhi-XXXXXXX-clean
    return { type: 'louhi', tag: rawTag.substring(31, 38) };
  } else {
    // Regular tag (tag:), return the remaining segment.
    return { type: 'tag', tag: rawTag.substring(4) };
  }
}

declare global {
  interface Window {
    perf: SkPerfConfig;
  }
}
