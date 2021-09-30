    USE ANDROID;
    DELETE
    FROM
        paramsets
    WHERE
        tile_number=283 AND
        param_key='sub_result' AND
        param_value>'showmap_granular' AND
        param_value<'showmap_granulas' LIMIT 1000;
