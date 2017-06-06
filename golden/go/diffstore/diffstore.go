// The diffstore package provides two implementations of the
// diff.DiffStore interface:
//
//    MemDiffStore allows to efficiently download and compare images from GS.
//    It caches diff results and images on disk and memory for high throughput.
//
//    NetDiffStore wraps a DiffStore instance (usually an instance of MemDiffStore)
//    in a gRPC service so the diff load can be distributed.
//
package diffstore
