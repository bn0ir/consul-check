consul-check
============

Control consul checks and trigger scripts on check fail.

config file
============

JSON used for config file.

Example config file:

```
{
    "PidFile": "path/to/pid/file",
    "LogFile": "path/to/log/file",
    "Consul": {
	  "Address": "127.0.0.1:8500",
	  "Scheme": "http"
    },
    "Operations":[
	{
	    "Key": "keyname",
	    "Script": "path/to/script.sh",
	    "Interval": checkinterval,
	    "Timeout": timeoutAfterScriptExec
	},
	{
	    "Key": "keyname",
	    "Script": "path/to/script.sh",
	    "Interval": checkinterval,
	    "Timeout": timeoutAfterScriptExec
	},
	...
    ]
}
```

If LogFile set to "", then log will be send to stdout.

commands
============

 - Update config file: kill -HUP PID

 - Stop process: kill PID