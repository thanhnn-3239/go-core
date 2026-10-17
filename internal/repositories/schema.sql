CREATE TABLE IF NOT EXISTS departments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    created_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS employees (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    age           INTEGER NOT NULL CHECK (age > 0),
    position      TEXT    NOT NULL,
    department_id INTEGER NOT NULL REFERENCES departments(id),
    salary        INTEGER NOT NULL CHECK (salary > 0),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    deleted_at    TEXT
);
CREATE INDEX IF NOT EXISTS idx_employees_department
    ON employees(department_id) WHERE deleted_at IS NULL;
