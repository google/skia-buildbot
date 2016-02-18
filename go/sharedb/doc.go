// ShareDB allows to set up a shared key-value store that exposes
// BoltDB over the network.
//
// It provides a simplified interface than using BoltDB locally.
// In particular it only provides atomicity for individual read and
// write operations, but not transactions across multiple I/O operations.
//
// Each data item is address by a triple: database,bucket,key.
// 'database' maps to a BoltDB database stored in a single file.
// 'bucket' and 'key' correspond directly to the analog concepts in BoltDB.
//
// Multiple applications can share a server. Each application simple
// defines it's own namespace by using a unique database name.
//
package sharedb
