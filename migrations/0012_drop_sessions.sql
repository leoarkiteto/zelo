-- Sessions moved to Redis (in-memory). The persistent table is no longer
-- used; see specs/008-in-memory-session-storage.
DROP TABLE IF EXISTS sessions;
