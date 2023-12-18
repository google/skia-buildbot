// Package midpoint encapsulates all logic pertaining to determining the next candidate for bisection.
//
// Bisection finds a culprit for some performance regression by continuously comparing two commits.
// The range of commits is reduced by half for every iteration, until one commit that causes a
// regression is identified.
//
// As part of that search, the next "candidate" to compare against is found by calculating the midpoint
// between two commits. Once all options have been exhausted (specifically, when the starting and
// ending commits are adjacent to each other), the system assumes a DEPS roll and searches for the
// culprit in the range of commits rolled in.
//
// For example, if we have C1 and C5:
// Gitiles Logs would return [C5, C4, C3, C2]. C5 is popped off, and we're left with
// [C4, C3, C2]. Because the ordering is from latest -> earliest, this is reversed, and
// thus becomes [C2, C3, C4]. The calculated midpoint would be at index 1, or C3.
//
// The next iterations would be as follows: C1 - C3 and C3 - C5.
// Gitiles Logs would return [C3, C2] and removing C3 leaves C2, which would be the next
// midpoint for that pair. Following the same logic, C4 would be the midpoint for C3 - C5.
//
// In the final iteration, we are left with adjacent pairs ie/ C1 - C2 // C2 - C3 ...
// It's assumed that there's been a DEPS roll between C1 and C2. DEPS file for each commit
// is unpacked.
//
// If there indeed was a DEPS roll, and thus the revision for a project (ie/ V8) has been rolled,
// the "dependency" field of the response will be non-nil, denoting the start, mid and end for the
// roll.
//
// For example, if between C1 and C2 there's a V8 roll, and if V8 is rolled from V1 to V3,
// the expected response would be:
//
//	Commit {
//	  gitHash: C1,
//	  repoisitoryUrl: "https://chromium.org/chromium/src"
//	  dependency: Dependency {
//	    repositoryUrl: V8,
//	    startGitHash: V1,
//	    endGitHash: V3,
//	    midGitHash: V2,
//	  }
//	}
package midpoint
