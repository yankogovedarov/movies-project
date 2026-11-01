package db

// NewMigrate exposes newMigrate to the external db_test package, so a test can
// stop the schema at an older version before running Migrate (Bug 27).
var NewMigrate = newMigrate
