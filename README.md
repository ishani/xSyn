![xSyn Logo](https://raw.githubusercontent.com/ishani/xSyn/master/logo.jpg)

Compact server implementing xBrowserSync API using Golang and BoltDB; supports API version 1.1.4 (Oct 2018)

Easy to deploy via Docker, xSyn provides a lean server for privately hosting your own bookmarks sync store. As of writing, [xBrowserSync](https://www.xbrowsersync.org/) is available for Chrome, Firefox, Android - It's really good!

### Configuring

xSyn pulls configuration from a TOML file during boot and allows environment variable overloads for all the values. Easy to setup and easy to tune.

Check `prod.toml` for all available settings and override names.

### Securing

xSyn can be run unsecured, with TLS via provided keys or automatically secured via *Let's Encrypt*. 

It is possible to run a special route that toggles the `Accepting New Syncs` value while running, so one can open/close the gates on a public server to limit users manually.

Rate-limiting is enabled by default on all routes and is easily configurable.

--

### DockerHub

An up-to-date build is available at [hdenholm/xsyn:latest](https://hub.docker.com/r/hdenholm/xsyn/)

Note that build dates are stamped into the published images, which you can view in the log on startup (with `release_mode` / `XS_SRV_RELEASE` set to false so you can see the Info logs)

### Azure

I have a test instance running on Azure using their slightly restrictive Docker support for App Services. 

In the portal, navigate to *Application Settings*, make sure `WEBSITES_ENABLE_APP_SERVICE_STORAGE` is enabled. With a default configuration file, add `WEBSITES_PORT` and set it to 80 - remember to update it if you set the `port` / `XS_SRV_PORT` config value.

Because Azure doesn't let you manually configure volume mapping, we have to override the default file for the BoltDB. Set `XS_BOLT_FILE` to `/home/site/store.db` (or anything under the `/home/site` folder)

Note that currently there is no way to use *Let's Encrypt* on Azure App Services as of writing as it requires more than one port to be exposed, and Azure doesn't allow this.

### AWS

xSyn works on ECS easily, including full *Let's Encrypt* support if you assign an EIP and map it to an owned domain.

I have a simple example task template [over here](https://gist.github.com/ishani/06a99050500069319493facd31b6576e) - tested, but not a lot. Note this has LE enabled, so either disable that or set your own domain name up before you kick it off.


--
#### Todo

* Tests

As it stands, xSyn works great for a private xBrowserSync server - I've been running with it for about a year - and I've poked it about on a few different platforms, but it really needs some actual tests and fuzzing done

