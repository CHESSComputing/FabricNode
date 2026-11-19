# syntax=docker/dockerfile:1

FROM golang:1.27 AS builder

WORKDIR /src

# Copy module/workspace files first to leverage layer caching
COPY Makefile ./
COPY pkg/ ./pkg/
COPY services/ ./services/
COPY scripts ./scripts

# Build all services
RUN make

# Runtime stage
FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy built service binaries and supporting directories
COPY --from=builder /src/services/catalog-service/bin/catalog-service /usr/local/bin/catalog-service
COPY --from=builder /src/services/data-service/bin/data-service /usr/local/bin/data-service
COPY --from=builder /src/services/identity-service/bin/identity-service /usr/local/bin/identity-service
COPY --from=builder /src/services/notification-service/bin/notification-service /usr/local/bin/notification-service
COPY --from=builder /src/services/ ./services/

EXPOSE 8781 8782 8783 8784

CMD ["catalog-service"]
