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
// Each iterative call to midpoint means that the bisection workflow is continuing to search for the culprit.
//
// EXAMPLE:
//
// Let's assume the most basic: Chromium @ commit hash 1 (C1) and C2.
// Gitiles Logs would return [C5, C4, C3, C2]. C5 is popped off, and we're left with
// [C4, C3, C2]. Because the ordering is from latest -> earliest, this is reversed, and
// thus becomes [C2, C3, C4]. The calculated midpoint would be at index 1, or C3.
//
// The next set of iterations would be as follows: C1 - C3 and C3 - C5.
// For the range C1 - C3, Gitiles Logs would return [C3, C2] and removing C3 leaves C2,
// which would be the next midpoint for that pair.
// Following the same logic, C4 would be the midpoint for C3 - C5.
// This presents the bisection workflow with the following set of iterations to compare:
// (C1 - C2 | C2 - C3 | C3 - C4 | C4 - C5)
//
// In this example, let's say C3 was a DEPS roll. A DEPS roll usually modifies the git hash
// for _one_ repository per roll in the DEPS file. That means if C2's DEPS file had V8 @ commit 1,
// a DEPS roll for the V8 repository would change C3 to V8 @ commit, say 3.
//
// As bisection continues to search for the culprit, the start and end commits may be C2 and C3.
// In this case, Gitiles Logs would return [C3], which is our indicator that the two commits
// are adjacent. For adjacent changes, FindMidCombinedCommit assumes C3 to be a DEPS roll.
//
// Note: FindMidCombinedCommit only supports git-based dependencies (no CIPD).
//
// When a DEPS roll is assumed, FindMidCombinedCommit fetches the DEPS content for each of the files
// and finds the first different repository (different = git hash is different for the same repository url,
// which would indicate "the roll" happened. Following the example above, if V8 was rolled
// from V8@1 to V8@3, FindMidCombinedCommit would determine V8@2 as midpoint because Gitiles logs
// for V8 would return [V8@3, V8@2], popping off V8@3 and leaving V8@2.
//
// If the two DEPS contents had been equal (meaning the DEPS roll assumption was wrong),
// FindMidCombinedCommit returns the former commit (C2), which is its indicator that there's nothing
// else to parse through.
//
// All git-based dependencies need to build on some Chromium base. For when V8@2 is determined as midpoint,
// FindMidCombinedCommit would return a CombinedCommit with C2 as Main, and [V8@2] as its ModifiedDeps.
//
// Continuing the example, let's assume bisect is still searching for the culprit. Since the last midpoint
// was C2 + V8@2, the comparisons then become (C2 - C2+V8@2 | C2+V8@2 - C3).
// On either arm of the comparison, ModifiedDeps is present, which is the indicator that the search is happening
// in some git-based dep repository (in this case V8).
//
// To determine the range of commits to send to Gitiles, the implementation requires the start/end to be present
// for that git-based dep repository. So, for C2 - C2+V8@2, it'll fill the left CombinedCommit by searching DEP
// at Chromium@2, find V8 and fill it. So C2 becomes C2+V8@1, and now it can compare V8@1 to V8@2.
//
// If V8@2 is a DEP roll as well, say rolled WebRTC (W for short in example) from commit 1 to 3,
// following the logic above, FindMidCombinedCommit would return a Combined Commit:
// Main: C2, Modified Deps: [V8@1, W@2].
//
// EDGE CASES:
//
// FindMidCombinedCommit will fill in the DEPS of commits and pass that information to a helper
// function to find the midpoint between two commits. This leads to a few edge cases when
// FindMidCombinedCommit compares commits with separate base hashes. Those edge cases are:
// - Given the second to last commit in a DEPS roll vs the DEPS roll, return the last commit in the
// DEPS roll. For example, given the commits: {C@1}; {C@1, skia@1}; {C@1, skia@2}; {C@1, skia@3};
// {C@2}, we want to find midpoint of {C@1, skia@2} vs {C@2}. {C@2} fills in DEPS and
// becomes {C@2, skia@3}. Instead of returning {C@1, skia@2}, FindMidCombinedCommit returns
// {C@1, skia3}.
// - Given the last commit in a DEPS roll vs the DEPS roll, return the last commit. For example,
// {C@1, skia@3} vs {C@2}. {C@2} fills in DEPS and becomes {C@2, skia@3}. Instead of treating
// this as skia@3 vs skia@3, instead return {C@1, skia@3}.
// - Given the first commit in a DEPS roll vs the previous base commit, return the base commit.
// For example, {C@1} vs {C@1, skia@1} should return {C@1}, using similar logic as the previous
// edge case.
//
// CAVEATS:
//   - The implementation does not support a DEPS roll that rolls more than 1 git-base dependency.
//     The implementation today will start digging the first one it finds, even though the actual culprit could
//     be in one of the other git-based dependencies that it rolls.
//   - From the example above, let's say V8@2 rolled W8 from 1 to 2 (meaning that's also adjacent).
//     It is possible that the W8 roll is also a DEPS roll, but the implementation today does not dig further.
//     It instead terminates that they're adjancent and is unable detemrine midpoint.
//   - FindMidCombinedCommit also does not support the scenario where there are more than 1 modified deps
//     in the two commits provided. FindMidCombinedCommit was implemented expecting linear growth of modified
//     dependencies, following the assumption that a DEPS roll only rolls one dependency at a time.
package midpoint
