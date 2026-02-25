# MariaDB MCP Server (Go)

A Model Context Protocol (MCP) server for MariaDB, written in Go. This server provides secure access to MariaDB databases with support for vector stores and semantic search.

## Features

- **11 MCP Tools**: Complete database and vector store operations
- **Security First**: MULTI_STATEMENTS disabled by default to prevent SQL injection
- **Multiple Transports**: stdio, SSE, and HTTP support
- **Vector Stores**: Native support for MariaDB's VECTOR type with indexing
- **Embedding Providers**: OpenAI and Google Gemini support
- **Read-Only Mode**: Optional enforcement of read-only queries

## Installation

### From Source

```bash
go install github.com/rahadiangg/mcp-mariadb/cmd/mcp-mariadb@latest
```

### Using Docker

```bash
docker pull ghcr.io/rahadiangg/mcp-mariadb:latest
```

## Configuration

Configuration is done via environment variables (all prefixed with `MCP_MARIADB_`):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HOST` | No | localhost | Database host |
| `PORT` | No | 3306 | Database port |
| `USER` | Yes | - | Database user |
| `PASSWORD` | Yes | - | Database password |
| `DATABASE` | No | - | Default database |
| `CHARSET` | No | - | Connection charset |
| `SSL` | No | false | Enable SSL |
| `SSL_CA` | No | - | SSL CA certificate path |
| `SSL_CERT` | No | - | SSL client certificate path |
| `SSL_KEY` | No | - | SSL client key path |
| `SSL_VERIFY_CERT` | No | true | Verify SSL certificate |
| `SSL_VERIFY_IDENTITY` | No | false | Verify SSL hostname |
| `READ_ONLY` | No | true | Enable read-only mode |
| `MAX_POOL_SIZE` | No | 10 | Maximum connection pool size |
| `EMBEDDING_PROVIDER` | No | - | Embedding provider: `openai` or `gemini` |
| `OPENAI_KEY` | No | - | OpenAI API key |
| `GEMINI_KEY` | No | - | Google Gemini API key |

### Example .env file

```bash
MCP_MARIADB_HOST=localhost
MCP_MARIADB_PORT=3306
MCP_MARIADB_USER=mariadb_user
MCP_MARIADB_PASSWORD=secure_password
MCP_MARIADB_DATABASE=mydb
MCP_MARIADB_READ_ONLY=true
MCP_MARIADB_MAX_POOL_SIZE=10

# Optional: For vector store features
MCP_MARIADB_EMBEDDING_PROVIDER=openai
MCP_MARIADB_OPENAI_KEY=sk-...
```

## Usage

### Claude Desktop

Add to your Claude Desktop config:

```json
{
  "mcpServers": {
    "mariadb": {
      "command": "mcp-mariadb",
      "env": {
        "MCP_MARIADB_HOST": "localhost",
        "MCP_MARIADB_PORT": "3306",
        "MCP_MARIADB_USER": "your_user",
        "MCP_MARIADB_PASSWORD": "your_password"
      }
    }
  }
}
```

### Command Line

```bash
# stdio transport (default)
mcp-mariadb --transport stdio

# SSE transport
mcp-mariadb --transport sse --host 0.0.0.0 --port 9001

# HTTP transport
mcp-mariadb --transport http --host 0.0.0.0 --port 9001 --path /mcp
```

### Docker

```bash
docker run -d \
  -e MCP_MARIADB_HOST=host.docker.internal \
  -e MCP_MARIADB_PORT=3306 \
  -e MCP_MARIADB_USER=root \
  -e MCP_MARIADB_PASSWORD=password \
  -e MCP_MARIADB_READ_ONLY=true \
  -p 9001:9001 \
  ghcr.io/rahadiangg/mcp-mariadb:latest \
  --transport sse --host 0.0.0.0
```

## Available Tools

### Database Tools (6)

| Tool | Description |
|------|-------------|
| `list_databases` | Lists all accessible databases |
| `list_tables` | Lists all tables in a database |
| `get_table_schema` | Gets schema (columns, types) for a table |
| `get_table_schema_with_relations` | Gets schema with foreign key relationships |
| `execute_sql` | Executes a read-only SQL query |
| `create_database` | Creates a new database |

### Vector Store Tools (5)

*Requires `EMBEDDING_PROVIDER` to be configured*

| Tool | Description |
|------|-------------|
| `create_vector_store` | Creates a table with indexed VECTOR column |
| `list_vector_stores` | Lists all vector stores in a database |
| `delete_vector_store` | Deletes a vector store table |
| `insert_docs_vector_store` | Inserts documents with embeddings |
| `search_vector_store` | Semantic search using cosine distance |

## Security

### MULTI_STATEMENTS Protection

This server **always** disables the `MULTI_STATEMENTS` flag in the MySQL driver, even if not explicitly specified. This prevents SQL injection via multiple statements like:

```sql
SELECT * FROM users WHERE id = 1; DROP TABLE users; --
```

Attempts to execute such queries will be rejected with an error.

### Read-Only Mode

When `READ_ONLY=true` (default), only queries starting with `SELECT`, `SHOW`, `DESCRIBE`, `USE`, or `EXPLAIN` are allowed. Write operations are blocked at the tool level.

## Development

### Prerequisites

- Go 1.23+
- MariaDB 10.6+ (for VECTOR type support)

### Building

```bash
go build ./cmd/mcp-mariadb
```

### Testing

```bash
go test ./...
```

## Project Structure

```
./
├── cmd/mcp-mariadb/main.go       # Entry point
├── internal/
│   ├── config/config.go          # Environment configuration
│   ├── database/
│   │   ├── pool.go               # Safe connection pool
│   │   └── driver.go             # Custom driver (MULTI_STATEMENTS disabled)
│   ├── mcp/
│   │   ├── server.go             # MCP server initialization
│   │   └── tools.go              # 11 tool implementations
│   └── embeddings/
│       ├── service.go            # Embedding service interface
│       ├── openai.go             # OpenAI provider
│       └── gemini.go             # Gemini provider
├── go.mod
├── Dockerfile
└── README.md
```

## License

MIT License - see LICENSE file for details
