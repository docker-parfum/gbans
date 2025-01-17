# Installation Guide

Basic installation overview of the gbans server and sourcemod plugin.

## Sourcemod Plugins

The following extensions must be installed for gbans to work, see their documentation for up to date installation
instructions:

- [System2](https://github.com/dordnung/System2) Provides HTTP(S) client functionality
- [sm-json](https://github.com/clugg/sm-json) `Required for development only` Provides JSON encoding/decoding.
- [Connect](https://github.com/asherkin/connect) Provides `OnClientPreConnectEx`

## gbans Server

Precompiled binaries will be provided once the project is in a more stable state.

- [make](https://www.gnu.org/software/make/) Not strictly required but provides predefined build commands
- [golang 1.16+](https://golang.org/) gbans is written in go. Version >=1.16 is *REQUIRED* due to using iofs embed features.
- [PostgreSQL](https://www.postgresql.org/) is used as the data store. Version 12 is the only version currently tested
against. However i believe anything 10 and up should work. Please let me know if this is not the case.
  - [PostGIS](https://postgis.net/) extension is also used for some GIS functionality.
- [NodeJS 14+](https://nodejs.org/en/) To build frontend 
  - [yarn](https://yarnpkg.com/) JS package manager

Basic steps to build the binary packages:

    1. git clone git@github.com:leighmacdonald/gbans.git && cd gbans
    2. make
 
You should now have a binary located at `./build/$platform/gbans`

## Configuration

### Server

Copy the example configuration `gbans_example.yml` and name it `gbans.yml`. It should be in
the same directory as the binary. Configure it as desired. 

#### Starting the server

To start the server just run `./gbans serve`. It should show output similar to the following if 
successful.

```
➜  gbans git:(master) ✗ ./gbans serve
INFO[0000] Using config file: gbans.yaml 
INFO[0000] Starting gbans service                       
DEBU[0000] Ban sweeper routine started                  
INFO[0000] Bot is now running.  Press CTRL-C to exit.   
INFO[0000] Connected to session ws API                  
```

It's recommended to create a [systemd .service](https://freedesktop.org/software/systemd/man/systemd.service.html) 
file so that it can start automatically. More info on configuring this will be available at a later
date.

### Sourcemod

Place the `sourcemod/plugins/gbans.smx` file into `tf/addons/sourcemod/plugins`. Then add the config as 
described below.

This config file should be places in `tf/addons/sourcemod/configs/gbans.cfg`.

```
"gbans"
{
	// Remote gban server host
	"host"	"https://gbans.example.com"

	// Remote gban server port
	"port"	"443"

	// Unique server name for this server, the same as a "server-id"
	"server_name"	"example-1"

	// The authentication token used to retrieve a auth token
	"server_key"	"YOUR_TOKEN"
}
```

The server gbans server is running you should now be able to see the `[GB]` message logs in the
console. With a message like below on successful authentication with the server.

```
[GB] Using config file: addons/sourcemod/configs/gbans.cfg
[GB] Request to https://gbans.example.com/v1/auth finished with status code 200 in 0.01 seconds
[GB] Successfully authenticated with gbans server
```
