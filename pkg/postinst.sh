#!/bin/sh

set -e

# Initial installation: $1 == 1
# Upgrade: $1 == 2

#if [ $1 -eq 1 ] ; then
  if ! getent group "statsite" > /dev/null 2>&1 ; then
    groupadd -r "statsite"
  fi
  if ! getent passwd "statsite" > /dev/null 2>&1 ; then
    useradd -r -g elk -d /usr/share/elk -s /sbin/nologin \
      -c "statsite user" elk
  fi

  mkdir -p /etc/statsd-ha-proxy
  mkdir -p /var/log/statsd-ha-proxy
  chown -R statsite:statsite /var/log/statsd-ha-proxy
  chmod 755 /var/log/statsd-ha-proxy

  if [ -x /bin/systemctl ] ; then
    /bin/systemctl daemon-reload
    /bin/systemctl enable statsd-ha-proxy.service
  elif [ -x /sbin/chkconfig ] ; then
    /sbin/chkconfig --add statsd-ha-proxy
  fi
#fi
