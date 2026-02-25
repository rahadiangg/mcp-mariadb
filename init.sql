-- Initialize test database with sample data

-- Grant additional privileges to test user for create_database tests
GRANT CREATE, ALTER, DROP, INDEX ON *.* TO 'mcpuser'@'%';
FLUSH PRIVILEGES;

-- Create test tables
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY (username)
);

CREATE TABLE IF NOT EXISTS posts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Insert sample data
INSERT INTO users (username, email) VALUES
    ('alice', 'alice@example.com'),
    ('bob', 'bob@example.com'),
    ('charlie', 'charlie@example.com')
    ON DUPLICATE KEY UPDATE username=username;

INSERT INTO posts (user_id, title, content) VALUES
    (1, 'First Post', 'Hello World! This is my first post.'),
    (1, 'Second Post', 'Learning about MariaDB and MCP.'),
    (2, 'Greetings', 'Hi everyone! Excited to be here.'),
    (3, 'Introduction', 'I am Charlie. Nice to meet you all.')
    ON DUPLICATE KEY UPDATE title=title;

-- Create a test database for vector operations
CREATE DATABASE IF NOT EXISTS vectordb;
