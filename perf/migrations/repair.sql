-- This finds all the trace_ids we need to clean up.
SELECT count(trace_id)
FROM Postings
WHERE
    tile_number=201
AND
    key_value > 'sub_result=SFSTATS_'
AND
    key_value < 'sub_result=SFSTATSa';

-- This means removing rows from:
-- ParamSets
-- Postings
--    of all traceids that match


select
    *
from
    ParamSets
WHERE
    param_value > 'SFSTATS_'
AND
    param_value < 'SFSTATSa'
AND
    tile_number=201
LIMIT 10;


-- Start by deleting from Postings.
DELETE
FROM Postings
WHERE
    tile_number=201
    AND
    trace_id IN (
        SELECT trace_id
        FROM Postings
        WHERE
            tile_number=201
        AND
            key_value > 'sub_result=SFSTATS_'
        AND
            key_value < 'sub_result=SFSTATSa'
        LIMIT 1000
    );

-- Then delete from ParamSets.

--  watch --interval 1 'cockroach sql --insecure --host=localhost:25000 --database android --echo-sql < repair-postings.sql'
