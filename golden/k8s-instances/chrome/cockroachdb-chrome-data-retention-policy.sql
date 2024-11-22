-- This SQL script is manually executed to apply data retention policy in
-- the database for the chrome instance.
USE chrome;

-- Add expire_at COLUMN that expires in 60 days by default
-- or never expires (1000 years) on update
ALTER TABLE expectations
    ADD COLUMN expire_at TIMESTAMPTZ
    DEFAULT now() + '60 days'
    ON UPDATE now() + '1000 years';

-- Enable row level TTL
ALTER TABLE expectations
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Update existing triaged records to never expire
UPDATE expectations
SET expire_at = expire_at + '1000 years'
WHERE expectation_record_id IS NOT null;

-- Add expire_at COLUMN that expires in 60 days whenever the row is updated
ALTER TABLE ValuesAtHead
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '60 days'
    ON UPDATE now() + '60 days';

-- Enable row level TTL
ALTER TABLE ValuesAtHead
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Add expire_at COLUMN that expires in 60 days whenever the row is updated
ALTER TABLE TiledTraceDigests
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '60 days'
    ON UPDATE now() + '60 days';

-- Enable row level TTL
ALTER TABLE TiledTraceDigests
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');

-- Add expire_at COLUMN that expires in 60 days whenever the row is updated
ALTER TABLE TraceValues
    ADD COLUMN expire_at TIMESTAMPTZ
        DEFAULT now() + '60 days'
    ON UPDATE now() + '60 days';

-- Enable row level TTL
ALTER TABLE TraceValues
SET (ttl_expiration_expression = 'expire_at', ttl_job_cron = '@daily');
