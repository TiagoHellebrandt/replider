# Postgres 16 + pgvector + wal-g (US-F002).
#
# The existing pg-backup sidecar writes a daily pg_dump to a Docker volume on
# the SAME VPS as the database. That protects against "someone dropped a
# table"; it protects against nothing that kills the machine — and the machine
# is a single VPS. wal-g adds the two properties a dump on the same disk can
# never have:
#
#   * OFF-SITE: base backups and WAL segments land in a GCS bucket;
#   * POINT-IN-TIME: continuous WAL archiving means recovery to any second,
#     not just to last night's 03:00 snapshot.
#
# Built on the same pgvector image the stack already runs, so the extension
# and the Postgres minor version stay identical to production.
FROM pgvector/pgvector:pg16

# wal-g ships static release binaries; TARGETARCH lets the same Dockerfile
# build on an arm64 laptop and an amd64 VPS.
ARG WALG_VERSION=v3.0.5
ARG TARGETARCH

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl; \
    case "${TARGETARCH}" in \
      amd64) WALG_ARCH=amd64 ;; \
      arm64) WALG_ARCH=aarch64 ;; \
      *) echo "unsupported arch ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/wal-g.tar.gz \
      "https://github.com/wal-g/wal-g/releases/download/${WALG_VERSION}/wal-g-pg-ubuntu-20.04-${WALG_ARCH}.tar.gz"; \
    tar -xzf /tmp/wal-g.tar.gz -C /tmp; \
    mv "/tmp/wal-g-pg-ubuntu-20.04-${WALG_ARCH}" /usr/local/bin/wal-g; \
    chmod +x /usr/local/bin/wal-g; \
    rm -rf /tmp/wal-g.tar.gz /var/lib/apt/lists/*; \
    apt-get purge -y curl; \
    apt-get autoremove -y; \
    wal-g --version

# Archiving is configured through the Postgres config, not the entrypoint, so
# it survives every restart path (including a crash-recovery start).
COPY postgresql.walg.conf /etc/postgresql/postgresql.walg.conf
COPY 00-enable-walg.sh /docker-entrypoint-initdb.d/00-enable-walg.sh
COPY docker-entrypoint-walg.sh /usr/local/bin/docker-entrypoint-walg.sh
COPY walg-base-backup.sh /usr/local/bin/walg-base-backup.sh
RUN chmod +x /docker-entrypoint-initdb.d/00-enable-walg.sh \
    /usr/local/bin/docker-entrypoint-walg.sh /usr/local/bin/walg-base-backup.sh

ENTRYPOINT ["docker-entrypoint-walg.sh"]
CMD ["postgres"]
