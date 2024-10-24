FROM debian:trixie-slim as os
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /simd

COPY ./simd-1 /usr/bin/simd
COPY ./start_simd_with_upgrade.sh /usr/bin/start_simd_with_upgrade.sh
RUN chmod +x /usr/bin/*

CMD ["start_simd_with_upgrade.sh", "--home", "/.simapp", "start"]
