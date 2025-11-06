FROM golang:1.24-alpine

WORKDIR /app

# Install Node.js 18
RUN apk add --no-cache nodejs npm

# Copy source code and vendor directory
COPY . .

# Build the dashboard binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o dashboard ./cmd/dashboard

# Build frontend
WORKDIR /app/frontend

# Copy source code and node_modules
COPY frontend/ .

# Set environment variables for React build
ENV REACT_APP_PUBLIC_DOMAIN=https://dashboard-public-ship-status.apps.ci.l2s4.p1.openshiftapps.com
ENV REACT_APP_PROTECTED_DOMAIN=https://dashboard-protected-ship-status.apps.ci.l2s4.p1.openshiftapps.com

# Build the application
RUN npm run build

# Move built files to static directory
RUN mkdir -p /app/static && cp -r build/* /app/static/

WORKDIR /app


# Expose port
EXPOSE 8080

# Run the dashboard server
CMD ["./dashboard"]
