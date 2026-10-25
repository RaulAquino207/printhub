# Development-only CUPS server with virtual queues that print to /dev/null.
# Used to exercise PrintHub's CUPS provider without printers. Not for production.
FROM debian:trixie-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends cups \
    && rm -rf /var/lib/apt/lists/*

COPY cupsd.conf /etc/cups/cupsd.conf
COPY virtual-mono.ppd virtual-color.ppd /etc/printhub/
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

# Virtual queues write to file:///dev/null, which CUPS refuses by default.
RUN echo "FileDevice Yes" >> /etc/cups/cups-files.conf \
    && chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 631
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
