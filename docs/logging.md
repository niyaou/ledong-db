# Logging

The service writes newline-delimited JSON logs to both standard output and a rolling file. Standard output remains available for the cloud runtime collector, while the file provides local retention on the server.

## Configuration

The defaults are equivalent to:

```yaml
server:
  log_level: info
  log_file: logs/server.log
  log_max_size_mb: 100
  log_max_backups: 10
  log_max_age_days: 30
  log_compress: true
  log_use_local_time: true
```

Every setting can be overridden with an environment variable:

```text
SERVER_LOG_LEVEL
SERVER_LOG_FILE
SERVER_LOG_MAX_SIZE_MB
SERVER_LOG_MAX_BACKUPS
SERVER_LOG_MAX_AGE_DAYS
SERVER_LOG_COMPRESS
SERVER_LOG_USE_LOCAL_TIME
```

For a Linux service, use an absolute path on persistent storage and grant the service account write access before startup, for example `/var/log/ledong-db/server.log`. Only one service process should write to a given rolling file; separate instances should use different file paths.

## Recorded events

- service initialization, dependency readiness, startup signal, and graceful shutdown
- every HTTP request with request ID, method, path, status, duration, response size, and remote address
- recovered HTTP panics with stack traces
- authentication rejection without recording the supplied credential
- critical business writes such as registration, recharge/refund, member-type changes, course changes, and pending-course admission
- SMS attempts and provider results, including the full phone number, Tencent request ID, serial number, status code, fee, and duration

SMS template parameters and Tencent credentials are deliberately excluded from logs.
