# Ship Status Dashboard

SHIP Status and Availability Dashboard monitor

## Project Structure

This project consists of multiple components:

- **Dashboard**: Web application for viewing and managing component status, availability, and outages
  - Backend: Go server (`cmd/dashboard`)
  - Frontend: React application (`frontend/`)
- **Component Monitor**: Monitoring service for tracking component health (not yet implemented)

## Dashboard Component

### Prerequisites

Before starting the dashboard, you must set up a PostgreSQL database:

1. Start a PostgreSQL container:
   ```bash
   podman run -d \
     --name ship-status-db \
     -e POSTGRES_PASSWORD=yourpassword \
     -p 5432:5432 \
     quay.io/enterprisedb/postgresql:latest
   ```

2. Create the database:
   ```bash
   podman exec ship-status-db psql -U postgres -c "CREATE DATABASE ship_status;"
   ```

3. Run migrations:
   ```bash
   go run ./cmd/migrate --dsn "postgres://postgres:yourpassword@localhost:5432/ship_status?sslmode=disable"
   ```

### Backend Setup

Start the local development environment (dashboard server and mock oauth-proxy):

```bash
make local-dev DSN="postgres://postgres:yourpassword@localhost:5432/ship_status?sslmode=disable"
```

This script:
- Starts the dashboard server on port 8080 (public route, no auth)
- Starts the mock oauth-proxy on port 8443 (protected route, requires basic auth)
- Sets up a user with credentials: `developer:password`
- Generates a temporary HMAC secret for request signing

### Frontend Setup

1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

3. Set environment variables (or use the .env.development file) and start the development server:
   ```bash
   REACT_APP_PUBLIC_DOMAIN=http://localhost:8080 \
   REACT_APP_PROTECTED_DOMAIN=http://localhost:8443 \
   npm start
   ```

The frontend will be available at `http://localhost:3000`.

## Component Monitor

The component monitor service is planned but not yet implemented. This component will be responsible for monitoring component health and status.

## Testing

### End-to-End Tests

Run the e2e test suite for the dashboard:

```bash
make e2e
```

The e2e script:
- Starts a PostgreSQL test container using podman
- Runs database migrations
- Starts the dashboard server on a dynamically assigned port (8080-8099)
- Starts the mock oauth-proxy on a dynamically assigned port (8443-8499)
- Executes the e2e test suite
- Cleans up all processes and containers on completion
