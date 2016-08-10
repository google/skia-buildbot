// package diffstore provides a distributed implementation of the
// diff.DiffStore interface.
//
// It consists of multiple components:
//
// - ImageLoader: Downloads images from Google storage and caches them on local
//                disk and in Redis. It aims that proactively fetching images
//                so that they are always in Redis when they are needed for
//                calculating diffs. Making real time diffs fast, because we
//                don't have to load anything from disk.
//
// TODO(stephana): Expand documentation below this line as the different
// parts are being added.
//
// - Differ:      TO BE ADDED.
//                Proactively caclculates diffs between images with the goal
//                of not having to calculate diffs when they are requested.
//                Results are cached in Redis and on disk.
//                Supports multiple diff metrics.
//
// Instances of the DiffStore can be run in a master/slave configuration
// topology where multiple slaves on other nodes point to the Redis instance
// on the master and the workload of diffing is evenly distributed.

package diffstore
