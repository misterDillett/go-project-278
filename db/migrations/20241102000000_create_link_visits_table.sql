-- +goose Up
CREATE TABLE IF NOT EXISTS link_visits (
                                           id SERIAL PRIMARY KEY,
                                           link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    ip VARCHAR(45),
    user_agent TEXT,
    referer TEXT,
    status INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
                                                                                             );

CREATE INDEX idx_link_visits_link_id ON link_visits(link_id);
CREATE INDEX idx_link_visits_created_at ON link_visits(created_at);

-- +goose Down
DROP TABLE IF EXISTS link_visits;