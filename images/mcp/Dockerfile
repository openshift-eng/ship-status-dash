FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

RUN microdnf install -y python3.12 python3.12-pip shadow-utils && microdnf clean all

WORKDIR /app

COPY mcp/requirements.txt .
RUN pip3.12 install --no-cache-dir -r requirements.txt

COPY mcp/api_client.py mcp/api_server.py ./

RUN useradd -r -u 10001 -g 0 -d /app mcp && chown -R 10001:0 /app

ENV MCP_TRANSPORT=streamable-http \
    MCP_HTTP_PORT=8090 \
    SHIP_STATUS_PUBLIC_API_URL=http://127.0.0.1:8080/api \
    SHIP_STATUS_PROTECTED_API_URL=http://127.0.0.1:8443/api \
    SHIP_STATUS_DASHBOARD_URL=http://127.0.0.1:8080/

EXPOSE 8090

USER 10001

CMD ["python3.12", "api_server.py"]
