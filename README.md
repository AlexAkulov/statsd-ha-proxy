# Statsd High Availability Proxy

```
                   |
         +-------------------+
         |     :8125 UDP     |
         |  statsd-ha-proxy  |
         +-------------------+
          /                 \
         / master            \ backup
+----------------+      +----------------+
|   :8125 TCP    |      |   :8125 TCP    |
|    statsite    |      |    statsite    |
+----------------+      +----------------+
```
