# xSyn
Compact server implementing xBrowserSync API using Golang and BoltDB; supports API version 1.1.4 (Oct 2018)

Easy to deploy via Docker, xSyn provides a lean server for privately hosting your own bookmarks sync store. As of writing, [xBrowserSync](https://www.xbrowsersync.org/) is available for Chrome, Firefox, Android and iOS. It's really good!

### DockerHub
A recent build is available at [hdenholm/xsyn:latest](https://hub.docker.com/r/hdenholm/xsyn/)

If running locally, ensure you map a volume to `/data` so that the BoltDB file persists.

### Azure
I have a test instance running on Azure using their slightly restrictive Docker support for App Services. 

In Application Settings, make sure `WEBSITES_ENABLE_APP_SERVICE_STORAGE` is enabled. With a default configuration file, add `WEBSITES_PORT` and set it to 8080. 

Because Azure doesn't let you manually configure volume mapping, we have to override the default file for the BoltDB. Set `XS_BOLT_FILE` to `/home/site/store.db` (or anything under the /home/site folder)

Check the `config.go` file for other envvars you can set to modify default behaviour.
