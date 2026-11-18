CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS products (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    category_id    INTEGER NOT NULL,
    name           TEXT NOT NULL,
    description    TEXT,
    price          NUMERIC NOT NULL CHECK (price >= 0),
    available_count INTEGER NOT NULL DEFAULT 0 CHECK (available_count >= 0),
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT
);