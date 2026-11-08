# e2e stand: a throwaway Debian "server" booted with systemd, running the real
# webtermin binary as a unit plus the services the panel manages. Built by
# e2e/run.sh from a staged context (binary + packaging files), never from the
# repo root.
FROM debian:trixie

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus cron ufw docker.io docker-cli docker-compose \
      openssh-server sudo curl ca-certificates iproute2 procps nginx
# apt lists are kept on purpose: the package-plan spec dry-runs `install htop`.

COPY webtermin /usr/bin/webtermin
COPY webtermin.service /lib/systemd/system/webtermin.service
COPY config.yaml /etc/webtermin/config.yaml
RUN chmod 0755 /usr/bin/webtermin \
 && chmod 0640 /etc/webtermin/config.yaml \
 && install -d -m 0700 /var/lib/webtermin \
 && systemctl enable webtermin docker cron ssh nginx

STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
