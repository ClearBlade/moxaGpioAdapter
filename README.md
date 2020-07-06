# moxa-gpio-adapter
The moxa-gpio-adapter allows the ClearBlade Edge/Platform to interact read and write Moxa GPIO values for the UC-8220 model gateways.

## Usage
```
TO-DO
```

## Command Line Arguments & Environment Variables
All ClearBlade Adapters require a certain set of System specific variables to start and connect with the ClearBlade Platform/Edge. These may be passed in either by command line arguments, or environment variables. Note that command line arguments take precedence over environment variables.

| Name | CLI Flag | Environment Variable | Default |
| --- | --- | --- | --- |
| System Key | `systemKey` | `CB_SYSTEM_KEY` | N/A |
| System Secret | `systemSecret` | `CB_SYSTEM_SECRET` | N/A |
| Platform/Edge URL | `platformURL` | N/A | `http://localhost:9000` |
| Platform/Edge Messaging URL | `messagingURL` | N/A | `localhost:1883` |
| Device Name (**depreciated**) | `deviceName` | N/A | `adapterName` provided when calling `adapter_library.ParseArguments` |
| Device Password/Active Key (**depreciated)** | `password` | N/A | N/A |
| Device Service Account | N/A | `CB_SERVICE_ACCOUNT` | N/A |
| Device Service Account Token | N/A | `CB_SERVICE_ACCOUNT_TOKEN` | N/A |
| Log Level | `logLevel` | N/A | `info` |
| Adapter Config Collection Name | `adapterConfigCollection` | N/A | `adapter_config` |

A System Key and System Secret will always be required to start the adapter, and it's recommended to always use a Device Service Account & Token for Adapters. 

Device Name and Password for Adapters are **depreciated** and only provided for backwards compatibility and should not be used for any new adapters.


## Adapter Configuration & Settings
This library does require the use of an Adapter Configuration Collection. This allows you to easily provide configuration options to your adapter at runtime via this Collection, rather than command line arguments, environment variables, or hardcoded values in your adapter. 

If your adapter does not have any specific settings it is still expected to have an entry in this collection, but adapter settings column can be left blank.

The default name for this Collection is `adapter_config`, and it's expected data structure is:

| Column Name | Column Type |
| --- | --- |
| `adapter_name` | `string` |
| `topic_root` | `string` |
| `adapter_settings` | `string` |

## Logging
This adapter introduces some basic logging level that your adapter can also leverage. Simply preface any of your log statements with the relevant level of the log surrounded with square brackets, and the `logLevel` flag passed in will be honored by your specific adapter logs. 

The supported log levels are `DEBUG`, `INFO`, `ERROR`, and `FATAL`.

# moxaGpioAdapter
