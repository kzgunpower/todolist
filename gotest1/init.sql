CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    done BOOLEAN DEFAULT FALSE
);
CREATE TABLE IF NOT EXISTS task_audit (
    id SERIAL PRIMARY KEY,
    action TEXT NOT NULL,
    task_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX ON task_audit (task_id);
CREATE INDEX ON task_audit (created_at);