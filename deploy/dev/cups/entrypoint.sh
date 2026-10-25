#!/bin/sh
# Starts cupsd, creates the virtual queues, then waits on cupsd.
set -eu

/usr/sbin/cupsd -f &
cupsd_pid=$!

until lpstat -r >/dev/null 2>&1; do sleep 0.2; done

lpadmin -p virtual-mono -E -v file:///dev/null -P /etc/printhub/virtual-mono.ppd \
  -D "Virtual mono (PrintHub dev)" -L "dev container"
lpadmin -p virtual-color -E -v file:///dev/null -P /etc/printhub/virtual-color.ppd \
  -D "Virtual color (PrintHub dev)" -L "dev container"
# Paused: accepts jobs but never processes them, to test pending jobs and cancellation.
lpadmin -p virtual-paused -E -v file:///dev/null -P /etc/printhub/virtual-mono.ppd \
  -D "Virtual paused (PrintHub dev)" -L "dev container"
cupsdisable virtual-paused

echo "CUPS ready: $(lpstat -p | tr '\n' ' ')"
wait "$cupsd_pid"
