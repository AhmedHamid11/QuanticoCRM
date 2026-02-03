#!/bin/bash

# FastCRM Services Manager
# Usage: ./services.sh [start|stop|status]

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_PID_FILE="$PROJECT_DIR/.backend.pid"
FRONTEND_PID_FILE="$PROJECT_DIR/.frontend.pid"

start_backend() {
    if [ -f "$BACKEND_PID_FILE" ] && kill -0 $(cat "$BACKEND_PID_FILE") 2>/dev/null; then
        echo "Backend already running (PID: $(cat $BACKEND_PID_FILE))"
        return 0
    fi

    echo "Starting backend..."
    cd "$PROJECT_DIR/backend"
    DATABASE_PATH="$PROJECT_DIR/fastcrm.db" nohup go run ./cmd/api/main.go > "$PROJECT_DIR/backend.log" 2>&1 &
    echo $! > "$BACKEND_PID_FILE"
    echo "Backend started (PID: $!)"
}

stop_backend() {
    if [ -f "$BACKEND_PID_FILE" ]; then
        PID=$(cat "$BACKEND_PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "Stopping backend (PID: $PID)..."
            kill $PID
            rm -f "$BACKEND_PID_FILE"
            echo "Backend stopped"
        else
            echo "Backend not running"
            rm -f "$BACKEND_PID_FILE"
        fi
    else
        # Try to find and kill by port (use full path for when called from Node.js)
        PID=$(/usr/sbin/lsof -t -i:8080 2>/dev/null)
        if [ -n "$PID" ]; then
            echo "Stopping backend (PID: $PID)..."
            kill $PID
            echo "Backend stopped"
        else
            echo "Backend not running"
        fi
    fi
}

start_frontend() {
    if [ -f "$FRONTEND_PID_FILE" ] && kill -0 $(cat "$FRONTEND_PID_FILE") 2>/dev/null; then
        echo "Frontend already running (PID: $(cat $FRONTEND_PID_FILE))"
        return 0
    fi

    echo "Starting frontend..."
    cd "$PROJECT_DIR/frontend"
    nohup npm run dev > "$PROJECT_DIR/frontend.log" 2>&1 &
    echo $! > "$FRONTEND_PID_FILE"
    echo "Frontend started (PID: $!)"
}

stop_frontend() {
    if [ -f "$FRONTEND_PID_FILE" ]; then
        PID=$(cat "$FRONTEND_PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "Stopping frontend (PID: $PID)..."
            kill $PID
            rm -f "$FRONTEND_PID_FILE"
            echo "Frontend stopped"
        else
            echo "Frontend not running"
            rm -f "$FRONTEND_PID_FILE"
        fi
    else
        # Try to find and kill by port (use full path for when called from Node.js)
        PID=$(/usr/sbin/lsof -t -i:5173 2>/dev/null)
        if [ -n "$PID" ]; then
            echo "Stopping frontend (PID: $PID)..."
            kill $PID
            echo "Frontend stopped"
        else
            echo "Frontend not running"
        fi
    fi
}

status() {
    echo "=== FastCRM Services Status ==="

    # Backend status (use full path for when called from Node.js)
    if [ -f "$BACKEND_PID_FILE" ] && kill -0 $(cat "$BACKEND_PID_FILE") 2>/dev/null; then
        echo "Backend:  RUNNING (PID: $(cat $BACKEND_PID_FILE)) - http://localhost:8080"
    elif /usr/sbin/lsof -i:8080 >/dev/null 2>&1; then
        echo "Backend:  RUNNING (Port 8080 in use) - http://localhost:8080"
    else
        echo "Backend:  STOPPED"
    fi

    # Frontend status (use full path for when called from Node.js)
    if [ -f "$FRONTEND_PID_FILE" ] && kill -0 $(cat "$FRONTEND_PID_FILE") 2>/dev/null; then
        echo "Frontend: RUNNING (PID: $(cat $FRONTEND_PID_FILE)) - http://localhost:5173"
    elif /usr/sbin/lsof -i:5173 >/dev/null 2>&1; then
        echo "Frontend: RUNNING (Port 5173 in use) - http://localhost:5173"
    else
        echo "Frontend: STOPPED"
    fi
}

case "$1" in
    start)
        start_backend
        start_frontend
        echo ""
        status
        ;;
    stop)
        stop_backend
        stop_frontend
        echo ""
        status
        ;;
    restart)
        stop_backend
        stop_frontend
        sleep 2
        start_backend
        start_frontend
        echo ""
        status
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
