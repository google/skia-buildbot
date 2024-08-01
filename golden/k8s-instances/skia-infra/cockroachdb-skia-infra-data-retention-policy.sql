-- This SQL script is manually executed to apply data retention policy in
-- the database for the skiainfra instance.
USE skiainfra;

-- Add expire_at COLUMN that expires in 90 days by default
-- or never expires (1000 years) on update
ALTER TABLE Expectations
    ADD COLUMN expire_at TIMESTAMPTZ
    DEFAULT now() + '90 days'
    ON UPDATE now() + '1000 years';

-- Enable row level TTL
ALTER TABLE Expectations
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Update expire_at for existing records
UPDATE Expectations
SET expire_at = expire_at + '1000 years'
WHERE expectation_record_id IS NOT null;

-- Add expire_at COLUMN that expires in 90 days whenever the row is updated
ALTER TABLE ValuesAtHead
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '90 days'
    ON UPDATE now() + '90 days';

-- Enable row level TTL
ALTER TABLE ValuesAtHead
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Add expire_at COLUMN that expires in 90 days whenever the row is updated
ALTER TABLE TiledTraceDigests
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '90 days'
    ON UPDATE now() + '90 days';

-- Enable row level TTL
ALTER TABLE TiledTraceDigests
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Add expire_at COLUMN that expires in 90 days whenever the row is updated
ALTER TABLE TraceValues
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '90 days'
    ON UPDATE now() + '90 days';

-- Enable row level TTL
ALTER TABLE TraceValues
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');
