#!/bin/sh

# Handle process termination
cleanup() {
    echo "Shutting down..."
    if [ -n "$APP_PID" ]; then
        kill $APP_PID 2>/dev/null
    fi
    if [ -n "$TUNNEL_PID" ]; then
        kill $TUNNEL_PID 2>/dev/null
    fi
    exit 0
}

# Set up signal handling
trap cleanup INT TERM

# Start the main application
/app/app &
APP_PID=$!

# Start cloudflared with the provided tunnel token
if [ -n "$TUNNEL_TOKEN" ]; then
    echo "Starting Cloudflare Tunnel..."
    /usr/local/bin/cloudflared tunnel --no-autoupdate run --token "$TUNNEL_TOKEN" &
    TUNNEL_PID=$!
    
    # Wait for either process to exit
    wait -n $APP_PID $TUNNEL_PID
    
    # If we get here, one of the processes died
    if ! kill -0 $APP_PID 2>/dev/null; then
        echo "Application crashed, shutting down tunnel..."
    else
        echo "Tunnel connection lost, attempting to restart..."
        cleanup
    fi
else
    echo "TUNNEL_TOKEN not provided, running without Cloudflare Tunnel"
    # Wait for the app process
    wait $APP_PID
fi

# If we get here, something went wrong
cleanup
