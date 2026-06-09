export function fuzzyScore(text: string, query: string): number {
  if (!query) return 0;
  if (!text) return -Infinity;

  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();

  if (lowerQuery === lowerText) return 100000; // Exact match

  let score = 0;
  let textIdx = 0;
  let prevMatchIdx = -1;
  let consecutive = 0;

  for (let i = 0; i < lowerQuery.length; i++) {
    const char = lowerQuery[i];
    // Find char in text starting from textIdx
    const matchIdx = lowerText.indexOf(char, textIdx);

    if (matchIdx === -1) return -Infinity; // Not found

    // Scoring
    if (matchIdx === prevMatchIdx + 1) {
      consecutive++;
      score += 10 * consecutive; // Consecutive bonus
    } else {
      consecutive = 0;
      score -= matchIdx - prevMatchIdx; // Distance penalty
    }

    // Start of word bonus
    if (matchIdx === 0) {
      score += 50; // Strong bonus for start of string
    } else if (/[^a-z0-9]/.test(lowerText[matchIdx - 1])) {
      score += 20; // Standard bonus for start of word boundary
    }

    textIdx = matchIdx + 1;
    prevMatchIdx = matchIdx;
  }

  // Tie-breaker: prefer shorter matches (closer to exact match)
  score -= (lowerText.length - lowerQuery.length) * 0.001;

  return score;
}

export function scoreParam(p: { key: string; value: string }, token: string): number {
  const eqIdx = token.indexOf('=');
  if (eqIdx === -1) {
    // Value only
    return fuzzyScore(p.value, token);
  } else {
    // Key=Value
    const kPart = token.substring(0, eqIdx);
    const vPart = token.substring(eqIdx + 1);

    let total = 0;

    if (kPart.length > 0) {
      const s = fuzzyScore(p.key, kPart);
      if (s === -Infinity) return -Infinity;
      total += s;
    }

    if (vPart.length > 0) {
      const hasGlobChar = vPart.includes('*') || vPart.includes('?') || vPart.includes(',');
      if (hasGlobChar) {
        try {
          const parts = vPart
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean);
          const regexes = parts.map((part) => {
            const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const pattern = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
            return new RegExp(pattern, 'i');
          });

          const matchesRegex = regexes.some((r) => r.test(p.value));
          if (!matchesRegex) return -Infinity;

          total += 1000; // Bonus for explicit glob match
        } catch (_e) {
          return -Infinity;
        }
      } else {
        const s = fuzzyScore(p.value, vPart);
        if (s === -Infinity) return -Infinity;
        total += s;
      }
    }

    return total;
  }
}

export function scoreParamAny(p: { key: string; value: string }, token: string): number {
  const eqIdx = token.indexOf('=');
  if (eqIdx === -1) {
    // Search BOTH Key and Value, return best match
    return Math.max(fuzzyScore(p.key, token), fuzzyScore(p.value, token));
  } else {
    // Fallback to strict if = is explicit
    return scoreParam(p, token);
  }
}
