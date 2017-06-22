package find_breaks

/*
This package attempts to find groups of failing tasks which constitute
particular breakages.

We start by merging failed tasks within a task spec into failure instances.
Note that we're assuming all failures within a task spec have the same cause;
a more intelligent system would be able to distinguish *what* failed and only
merge the failures if they are really related. Consider that future work.

For each contiguous set of failing tasks:
- Find the slice of commits which may have caused the break.
- Find the slice of commits which may have fixed the break.
- Fill in the slice of commits which are failing because of the break.

Then, we merge failures into FailureGroup instances. Two failures may be merged
if:
- The slices of failing commits overlap.
- The slices of commits which may have caused each failure overlap.
- The slices of commits which may have fixed each failure (if any) overlap.
- The slice of commits which may have fixed one failure cannot be wholly
  contained within the set of commits which are failing in the other.

As we merge failures, we may reduce the sizes of the slices of commits which
may have broken or fixed the failure. This is desired because the ultimate goal
is to narrow down the cause of failure to single commit. However, if we combine
failures in the wrong order, we may find that one odd failure prevents us from
merging with others which should be part of the FailureGroup. Therefore, we have
to consider every permutation of the failure set, and a failure might be found
in more than one FailureGroup.

Edge cases. A failure may extend up to the most recent task, or back
to before our time window, such that we can't be sure of which commits may have
caused the failure or fixed it. We have to treat these cases as if *any*
previous commit may have caused the failure, or any subsequent commit might have
fixed the failure. Additionally, we may have gaps due to still-running tasks or
those which incurred a mishap. We need to include or exclude those gaps as
appropriate.
*/
