-- Rollback SQL (For manual reference):
-- ALTER TABLE Autobisections DROP COLUMN workflow_id;
-- ALTER TABLE Autobisections DROP COLUMN regression_status;
-- ALTER TABLE Autobisections ADD COLUMN is_real_regression BOOL;

ALTER TABLE Autobisections DROP COLUMN is_real_regression;
-- insignificant / no culprit / has culprits
ALTER TABLE Autobisections ADD COLUMN regression_status TEXT;
ALTER TABLE Autobisections ADD COLUMN workflow_id TEXT;
