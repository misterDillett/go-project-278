-- name: CreateLink :one
INSERT INTO links (original_url, short_name, short_url)
VALUES ($1, $2, $3)
    RETURNING id, original_url, short_name, short_url, created_at;

-- name: GetLink :one
SELECT id, original_url, short_name, short_url, created_at
FROM links
WHERE id = $1;

-- name: GetLinkByShortName :one
SELECT id, original_url, short_name, short_url, created_at
FROM links
WHERE short_name = $1;

-- name: ListLinks :many
SELECT id, original_url, short_name, short_url, created_at
FROM links
ORDER BY id
    LIMIT $1 OFFSET $2;

-- name: CountLinks :one
SELECT COUNT(*) FROM links;

-- name: UpdateLink :one
UPDATE links
SET original_url = $2, short_name = $3, short_url = $4
WHERE id = $1
    RETURNING id, original_url, short_name, short_url, created_at;

-- name: DeleteLink :exec
DELETE FROM links WHERE id = $1;

-- name: CreateLinkVisit :one
INSERT INTO link_visits (link_id, ip, user_agent, referer, status)
VALUES ($1, $2, $3, $4, $5)
    RETURNING id, link_id, ip, user_agent, referer, status, created_at;

-- name: GetLinkVisits :many
SELECT id, link_id, ip, user_agent, referer, status, created_at
FROM link_visits
ORDER BY created_at DESC
    LIMIT $1 OFFSET $2;

-- name: CountLinkVisits :one
SELECT COUNT(*) FROM link_visits;

-- name: GetLinkVisitsByLinkID :many
SELECT id, link_id, ip, user_agent, referer, status, created_at
FROM link_visits
WHERE link_id = $1
ORDER BY created_at DESC
    LIMIT $2 OFFSET $3;

-- name: CountLinkVisitsByLinkID :one
SELECT COUNT(*) FROM link_visits WHERE link_id = $1;