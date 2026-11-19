package db

// NewMigrate exposes newMigrate to the external db_test package, so a test can
// stop the schema at an older version before running Migrate (Bug 27).
var NewMigrate = newMigrate

// SyncProgressEvery lets a test lower how often SyncScanResults logs progress.
var SyncProgressEvery = &syncProgressEvery
