

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