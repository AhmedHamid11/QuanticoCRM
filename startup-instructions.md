# FastCRM Startup Instructions

## Quick Start

From the `fastcrm` directory, use the services script:

```bash
./services.sh start
```

## Available Commands

| Command | Description |
|---------|-------------|
| `./services.sh start` | Start both backend and frontend |
| `./services.sh stop` | Stop both services |
| `./services.sh restart` | Stop and restart both services |
| `./services.sh status` | Check if services are running |

## Service URLs

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080

## Logs

Log files are located in the `fastcrm` directory:

- `backend.log` - Backend server logs
- `frontend.log` - Frontend dev server logs

## Manual Start (Alternative)

If you prefer to run services manually in separate terminals:

**Backend:**
```bash
cd backend
DATABASE_PATH="../fastcrm.db" go run ./cmd/api/main.go
```

**Frontend:**
```bash
cd frontend
npm run dev
```
