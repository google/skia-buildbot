package buildapi

// Get the last buildid stored.
// Scan backwards using the API until that buildid is found.
// Roll up a list of all new buildids, in order.
// Add each one to the repo.
