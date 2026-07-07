export const MAX_SUGGESTIONS = 100;
export const EXACT_MATCH_SCORE = 100000;

const PRIORITY_BOOST_MULTIPLIER = 10;

export const MAX_CACHE_SIZE = 1000;
export const globRegexCache = new Map<string, RegExp[]>();

const LAST_SEGMENT_BOOST = 100;

function getStartBoundaryBonus(text: string, lowerText: string, startIdx: number): number {
  if (startIdx === 0) return 50;
  if (/[^a-z0-9]/.test(lowerText[startIdx - 1])) return 30;
  if (/[A-Z]/.test(text[startIdx])) return 25;
  return 0;
}

function scoreFromStartPos(
  text: string,
  lowerText: string,
  lowerQuery: string,
  startIdx: number
): number {
  let score = getStartBoundaryBonus(text, lowerText, startIdx);

  // The first character matched at startIdx.
  // Initialize state for the match of the first character.
  let textIdx = startIdx + 1;
  let prevMatchIdx = startIdx;
  let consecutive = 1; // We have matched 1 character consecutively so far.

  for (let i = 1; i < lowerQuery.length; i++) {
    const char = lowerQuery[i];
    const matchIdx = lowerText.indexOf(char, textIdx);
    if (matchIdx === -1) return -Infinity;

    if (matchIdx === prevMatchIdx + 1) {
      consecutive++;
      score += 10 * consecutive; // Consecutive bonus
    } else {
      consecutive = 0;
      score -= matchIdx - prevMatchIdx; // Distance penalty
    }

    // Word boundary bonus during matching
    if (matchIdx > 0 && /[^a-z0-9]/.test(lowerText[matchIdx - 1])) {
      score += 15; // Standard bonus for start of word boundary
    } else if (matchIdx > 0 && /[A-Z]/.test(text[matchIdx])) {
      score += 10; // Bonus for CamelCase capital
    }

    textIdx = matchIdx + 1;
    prevMatchIdx = matchIdx;
  }

  // Tie-breaker: prefer shorter matches (closer to exact match)
  return score - (lowerText.length - lowerQuery.length) * 0.001;
}

function fuzzyScoreSingle(text: string, query: string): number {
  if (!query) return 0;
  if (!text) return -Infinity;

  const lowerQuery = query.toLowerCase();
  const lowerText = text.toLowerCase();

  if (lowerQuery === lowerText) return EXACT_MATCH_SCORE;

  const firstChar = lowerQuery[0];
  let pos = lowerText.indexOf(firstChar);
  if (pos === -1) return -Infinity;

  let bestScore = -Infinity;

  while (pos !== -1) {
    const score = scoreFromStartPos(text, lowerText, lowerQuery, pos);
    if (score > bestScore) {
      bestScore = score;
    }
    pos = lowerText.indexOf(firstChar, pos + 1);
  }

  return bestScore;
}

export function fuzzyScore(text: string, query: string): number {
  if (!query) return 0;
  if (!text) return -Infinity;

  const fullScore = fuzzyScoreSingle(text, query);

  const lastDotIdx = text.lastIndexOf('.');
  if (lastDotIdx !== -1 && lastDotIdx < text.length - 1) {
    const lastSegment = text.substring(lastDotIdx + 1);
    const segmentScore = fuzzyScoreSingle(lastSegment, query);
    if (segmentScore > -Infinity) {
      return Math.max(fullScore, segmentScore + LAST_SEGMENT_BOOST);
    }
  }

  return fullScore;
}

function scoreGlobValue(value: string, vPart: string): number {
  try {
    let regexes = globRegexCache.get(vPart);
    if (!regexes) {
      const parts = vPart
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      regexes = parts.map((part) => {
        const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const pattern = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
        return new RegExp(pattern, 'i');
      });

      if (globRegexCache.size >= MAX_CACHE_SIZE) {
        const oldestKey = globRegexCache.keys().next().value;
        if (oldestKey !== undefined) {
          globRegexCache.delete(oldestKey);
        }
      }
      globRegexCache.set(vPart, regexes);
    } else {
      // Move to end to mark as recently used (LRU)
      globRegexCache.delete(vPart);
      globRegexCache.set(vPart, regexes);
    }

    const matchesRegex = regexes.some((r) => r.test(value));
    return matchesRegex ? 1000 : -Infinity;
  } catch (_e) {
    return -Infinity;
  }
}

function scoreKeyValueToken(
  p: { key: string; value: string },
  kPart: string,
  vPart: string
): number {
  let total = 0;
  if (kPart.length > 0) {
    const s = fuzzyScore(p.key, kPart);
    if (s === -Infinity) return -Infinity;
    total += s;
  }

  if (vPart.length > 0) {
    const hasGlobChar = vPart.includes('*') || vPart.includes('?') || vPart.includes(',');
    const s = hasGlobChar ? scoreGlobValue(p.value, vPart) : fuzzyScore(p.value, vPart);
    if (s === -Infinity) return -Infinity;
    total += s;
  }

  return total;
}

function getParamBoost(key: string, includeParams?: string[] | null): number {
  if (includeParams && includeParams.length > 0) {
    const idx = includeParams.indexOf(key);
    if (idx !== -1) {
      // Higher priority (earlier in list) gets higher boost.
      return (includeParams.length - idx) * PRIORITY_BOOST_MULTIPLIER;
    }
  }
  return 0;
}

export function scoreParam(
  p: { key: string; value: string },
  token: string,
  includeParams?: string[] | null
): number {
  const eqIdx = token.indexOf('=');
  if (eqIdx === -1) {
    let score = fuzzyScore(p.value, token);
    if (score > -Infinity) {
      score += getParamBoost(p.key, includeParams);
    }
    return score;
  }

  return scoreKeyValueToken(p, token.substring(0, eqIdx), token.substring(eqIdx + 1));
}

export function scoreParamAny(
  p: { key: string; value: string },
  token: string,
  includeParams?: string[] | null
): number {
  const eqIdx = token.indexOf('=');
  if (eqIdx === -1) {
    let valScore = fuzzyScore(p.value, token);
    if (valScore > -Infinity) {
      valScore += getParamBoost(p.key, includeParams);
    }
    const keyScore = fuzzyScore(p.key, token);
    return Math.max(keyScore, valScore);
  }
  return scoreParam(p, token, includeParams);
}
