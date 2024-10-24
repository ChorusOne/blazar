FROM debian:trixie-slim as os
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /simd

COPY ./simd-2 /usr/bin/simd
RUN chmod +x /usr/bin/*

CMD ["simd", "--home", "/.simapp", "start"]
