<!--- GENERATED BY gomplate from scripts/docs/templates/monitor-page.md.tmpl --->

# logstash-tcp

Monitor Type: `logstash-tcp` ([Source](https://github.com/signalfx/signalfx-agent/tree/master/internal/monitors/logstash/tcp))

**Accepts Endpoints**: **Yes**

**Multiple Instances Allowed**: Yes

## Overview

Fetches events from the [logstash tcp output
plugin](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-tcp.html)
operating in either `server` or `client` mode and converts them to SignalFx
datapoints.  It is meant to be used in conjunction with the Logstash
[Metrics filter
plugin](https://www.elastic.co/guide/en/logstash/current/plugins-filters-metrics.html)
that turns events into metrics.

You can only use auto-discovery when this monitor is in `client` mode.

<!--- SETUP --->
### Example Logstash Config

This is a somewhat contrived example that shows the use of both `timer` and
`meter` metrics from the Logstash Metrics filter plugin:

```
input {
  file {
    path => "/var/log/auth.log"
    start_position => "beginning"
    tags => ["auth_log"]
  }

  # A contrived file that contains timing messages
  file {
    path => "/var/log/durations.log"
    tags => ["duration_log"]
    start_position => "beginning"
  }
}

filter {
  if "duration_log" in [tags] {
    dissect {
      mapping => {
        "message" => "Processing took %{duration} seconds"
      }
      convert_datatype => {
        "duration" => "float"
      }
    }
    if "_dissectfailure" not in [tags] { # Filter out bad events
      metrics {
        timer => { "process_time" => "%{duration}" }
        flush_interval => 10
        # This makes the timing stats pertain to only the previous 5 minutes
        # instead of since Logstash last started.
        clear_interval => 300
        add_field => {"type" => "processing"}
        add_tag => "metric"
      }
    }
  }
  # Count the number of logins via SSH from /var/log/auth.log
  if "auth_log" in [tags] and [message] =~ /sshd.*session opened/ {
    metrics {
      # This determines how often metric events will be sent to the agent, and
      # thus how often datapoints will be emitted.
      flush_interval => 10
      # The name of the meter will be used to construct the name of the metric
      # in SignalFx.  For this example, a datapoint called `logins.count` would
      # be generated.
      add_tag => "metric"
    }
  }
}

output {
  # This can be helpful to debug
  stdout { codec => rubydebug }

  if "metric" in [tags] {
    tcp {
      port => 8900
      # The agent will connect to Logstash
      mode => "server"
      # Needs to be '0.0.0.0' if running in a container.
      host => "127.0.0.1"
    }
  }
}
```


## Configuration

To activate this monitor in the Smart Agent, add the following to your
agent config:

```
monitors:  # All monitor config goes under this key
 - type: logstash-tcp
   ...  # Additional config
```

**For a list of monitor options that are common to all monitors, see [Common
Configuration](../monitor-config.md#common-configuration).**


| Config option | Required | Type | Description |
| --- | --- | --- | --- |
| `host` | **yes** | `string` | If `mode: server`, the local IP address to listen on.  If `mode: client`, the Logstash host/ip to connect to. |
| `port` | no | `integer` | If `mode: server`, the local port to listen on.  If `mode: client`, the port of the Logstash TCP output plugin.  If port is `0`, a random listening port is assigned by the kernel. (**default:** `0`) |
| `mode` | no | `string` | Whether to act as a `server` or `client`.  The corresponding setting in the Logtash `tcp` output plugin should be set to the opposite of this. (**default:** `client`) |
| `desiredTimerFields` | no | `list of strings` |  (**default:** `[mean max p99 count]`) |
| `reconnectDelay` | no | `int64` | How long to wait before reconnecting if the TCP connection cannot be made or after it gets broken. (**default:** `5s`) |
| `debugEvents` | no | `bool` | If true, events received from Logstash will be dumped to the agent's stdout in deserialized form (**default:** `false`) |




