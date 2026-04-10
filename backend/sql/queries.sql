-- name: CreateEdit :one
INSERT INTO edits (
    name,
    author,
    prompt,
    start_lng,
    start_lat,
    end_lng,
    end_lat,
    image_path
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetEdit :one
SELECT * FROM edits
WHERE id = ?
LIMIT 1;

-- name: ListEdits :many
SELECT * FROM edits
ORDER BY created_at DESC;

-- name: ListEditsByAuthor :many
SELECT * FROM edits
WHERE author = ?
ORDER BY created_at DESC;

-- name: DeleteEdit :exec
DELETE FROM edits
WHERE id = ?;
