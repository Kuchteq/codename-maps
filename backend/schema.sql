CREATE TABLE IF NOT EXISTS edits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    author      TEXT    NOT NULL,
    prompt      TEXT    NOT NULL,

    -- GeoJSON Point: start position (WGS84)
    start_lng   REAL    NOT NULL,
    start_lat   REAL    NOT NULL,

    -- GeoJSON Point: end position (WGS84)
    end_lng     REAL    NOT NULL,
    end_lat     REAL    NOT NULL,

    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),

    -- Path to the generated noise PNG file
    image_path  TEXT    NOT NULL DEFAULT ''
);
