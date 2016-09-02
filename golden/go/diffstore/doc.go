// package diffstore provides an implementation of the diff.DiffStore interface.
//
// It consists of multiple components:
//
// - ImageLoader: Downloads images from Google storage and caches them on local
//                disk and RAM. It aims that proactively fetching images
//                so that they are always in RAM when they are needed for
//                calculating diffs. Making real time diffs fast, because we
//                don't have to load anything from disk.
//
// - Differ:      Proactively caclculates diffs between images with the goal
//                of not having to calculate diffs when they are requested.
//                Results are cached in RAM and on disk.
//                Supports multiple diff metrics.
//

package diffstore
