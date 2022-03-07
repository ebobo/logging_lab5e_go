# Logging in Healthy Buildings Backend

The logging package uses Über's Zap logging library (<https://github.com/uber-go/zap>). We chose this library because it is well maintained, widely used and has good performance.

## Configuration

When you run the logging package without any configuration you will get:

- log output on the console
- it will not write logs to files

The default log level for the healthy buildings backend is INFO.

Configuration is done through the environment variables.

### `HBB_LOGGER`

This can have two values:

- "file" - which means we log to files, but not on the console
- "both" - which means that we log to the console

The default is to log to console only.

### `HBB_LOG_DIR`

This variable controls which directory we send the log messages to. If this is unset we log into the "log" directory in the current working directory. If the directory path does not exist it will be created.

### `HBB_LOG_FILE_SIZE_MB`

The maximum log file size in megabytes. When this size is reached the log file is closed, renamed to reflect the date when it was rotated and compressed.

### `HBB_LOG_FILE_MAX_AGE_DAYS`

How many days to keep log files. If this is set to 0 we never delete log files. The default number of days is 90, but make sure to check this value in the source (pkg/logging.go) in case someone decides to change it.

## Code conventions

The code for logging is in the `pkg/logging` package.

Loggers are named `lg` throughout the project since it is short, easy to type and somewhat unambiguous (you are not going to confuse it with natural logarithms given the context and arguments given to it :)).

By convention, in this project, we instantiate the logger per package in the source file `lg.go`. This way you don't have to wonder where the logger instance comes from (which file initializes it) and if we were to change it later you can easily figure out what files you need to change.

Per default the `lg` instance points to a _Sugared_ logger, which means, it has a lot more convenience methods than just the naked ZAP logger. While slower, it is still faster than most logging libraries. You can find the documentation for this API at <https://pkg.go.dev/go.uber.org/zap#SugaredLogger>.

When adding log messages please think about the log levels used. _In general you should seek to minimize the logging to what's necessary and useful even when issuing debug log messages_.

## Changing logging level runtime

You can change the log level at runtime with the command line interface. In order to ask for the current log level you can use the `loglevel` subcommand.

```sh
$ bin/hbb loglevel get
INFO
```

You can temporarily alter the loglevel:

```sh
$ bin/hbb loglevel set --level debug --duration 15m
set loglevel to DEBUG for 300 seconds
```

...which will set the current log level to `DEBUG` for the next 15 minutes.

### Changing logging level via REST and gRPC

To query the current loglevel you can issue a GET to the `/api/v1/system/loglevel` endpoint. This will return the current loglevel, and for convenience, a list of valid log levels.

```json
{
  "logLevel": "DEBUG",
  "validLoglevels": ["DEBUG", "INFO", "WARN", "ERROR"]
}
```

You can change the log level temporarily by issuing a POST request to the `/api/v1/system/loglevel` endpoint with a JSON payload that specifies a loglevel and a duration:

```json
{
  "logLevel": "DEBUG",
  "durationSeconds": 1000
}
```

Since `durationSeconds` has some internal hard limits (right now you can at most request an increase in loglevel for 1 hour), this call will respond with a JSON message that has the same structure and which indicates how long the HBB will wait until it resets to the default log level again.

Note that multiple calls to this with different durations will favor the call with the shortest duration.

You can also use the generated gRPC method `client.System.SetLogLevel()` to set the log level. You can see an example of how it is used in the `cmd/hbb/loglevel_cmd.go` source file.
