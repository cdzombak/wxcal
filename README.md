# wxcal

wxcal generates iCal feeds for a given location: a weather forecast feed built from weather.gov forecast data, and/or a sunrise/sunset feed. At least one of the two must be requested; each is optional.

The forecast feed has an all-day event for today and for each of the following 6 days; each event contains a summary of the forecast for that day along with the day's sunrise & sunset times.

The sunrise/sunset feed has an all-day event per day giving that day's sunrise and sunset times. It covers 7 days by default, and up to any number of days via `-sunDays`, since sunrise/sunset times are calculated locally rather than fetched from weather.gov. Requesting only this feed makes no network requests at all, so it works for locations outside weather.gov's coverage area.

For an example feed generated with this tool, see [dzombak.com/local/wxcal/Ann-Arbor-MI.ics](https://www.dzombak.com/local/wxcal/Ann-Arbor-MI.ics).

## Usage

```text
wxcal [-flag value] [...]

  -calDomain string
        The calendar's domain (eg. "ical.dzombak.com") (required)
  -calLocation string
        The name of the calendar's location (eg. "Ann Arbor, MI") (required)
  -evtTitlePrefix string
        An optional prefix to be inserted before each event's title
  -icalFile string
        Path/filename for the weather forecast iCal output file (at least one of -icalFile/-sunIcalFile is required)
  -lat float
        The forecast location's latitude (eg. "42.27") (default 42.27)
  -lon float
        The forecast location's longitude (eg. "-83.74") (default -83.74)
  -sunDays int
        The number of days, counting today, to include in the sunrise/sunset calendar (requires -sunIcalFile) (default 7)
  -sunIcalFile string
        Path/filename for the sunrise/sunset iCal output file (at least one of -icalFile/-sunIcalFile is required)
  -timezone string
        IANA timezone name for the sunrise/sunset times in both calendars (eg. "America/Detroit"); if omitted, the timezone is determined from the forecast API or the given lat/lon
```

Additionally, `wxcal -version` will print the version number and exit.

### Example

This invocation, run periodically via cron, generates the example feed mentioned above ([dzombak.com/local/wxcal/Ann-Arbor-MI.ics](https://www.dzombak.com/local/wxcal/Ann-Arbor-MI.ics)):

```shell
wxcal -calDomain ics.dzombak.com -calLocation "Ann Arbor, MI" -lat 42.27 -lon "-83.74" -icalFile "/home/cdzombak/wxcal/public/Ann-Arbor-MI.ics" -evtTitlePrefix "[A2]"
```

This invocation generates two feeds, [one for Chelsea weather](https://www.dzombak.com/local/wxcal/Chelsea-MI.ics) and [another for Chelsea sunrise/sunset](https://www.dzombak.com/local/wxcal/Chelsea-MI-Sun.ics):

```shell
wxcal -calDomain ics.dzombak.com -calLocation "Chelsea, MI" -lat 42.35 -lon "-84.03" -icalFile "/home/cdzombak/wxcal/public/Chelsea-MI.ics" -sunIcalFile "/home/cdzombak/wxcal/public/Chelsea-MI-Sun.ics"
```

This invocation generates only a sunrise/sunset feed, covering a full year. Because no forecast is requested, it makes no network requests:

```shell
wxcal -calDomain ics.dzombak.com -calLocation "Chelsea, MI" -lat 42.35 -lon "-84.03" -sunIcalFile "/home/cdzombak/wxcal/public/Chelsea-MI-Sun.ics" -sunDays 365
```

### Timezones

Sunrise/sunset times are given in the location's local time, accounting for daylight saving time changes. This applies to both calendars: the forecast feed's events include the day's sunrise & sunset alongside the forecast.

wxcal determines that timezone from the `-timezone` flag if given; otherwise from the weather.gov API if a forecast calendar is being generated; otherwise by looking up the given latitude/longitude in an embedded [timezone boundary database](https://github.com/ringsaturn/tzf).

For days on which the sun does not rise or set at all — which happens above the Arctic Circle and below the Antarctic Circle — the sunrise/sunset calendar notes that the sun is up or down for the whole day.

## Installation

### macOS via Homebrew

```shell
brew install cdzombak/oss/wxcal
```

### Debian via apt repository

Install my Debian repository if you haven't already:

```shell
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://dist.cdzombak.net/deb.key | sudo gpg --dearmor -o /etc/apt/keyrings/dist-cdzombak-net.gpg
sudo chmod 0644 /etc/apt/keyrings/dist-cdzombak-net.gpg
echo -e "deb [signed-by=/etc/apt/keyrings/dist-cdzombak-net.gpg] https://dist.cdzombak.net/deb/oss any oss\n" | sudo tee -a /etc/apt/sources.list.d/dist-cdzombak-net.list > /dev/null
sudo apt-get update
```

Then install `wxcal` via `apt-get`:

```shell
sudo apt-get install wxcal
```

### Manual installation from build artifacts

Pre-built binaries for Linux and macOS on various architectures are downloadable from each [GitHub Release](https://github.com/cdzombak/wxcal/releases). Debian packages for each release are available as well.

### Build and install locally

Clone the repo (`https://github.com/cdzombak/wxcal.git`) and change into the wxcal source directory.

Run `make install` to install `wxcal` to `/usr/local/bin`. Or, to install somewhere else, run `make build` and move `./out/wxcal` to wherever you'd like.

### Uninstallation

Just remove the `wxcal` binary from wherever it's installed. If you installed to `/usr/local/bin` with `make install`, run `make uninstall` to remove it.

## Docker Images

Docker images are available for a variety of Linux architectures from [Docker Hub](https://hub.docker.com/r/cdzombak/wxcal) and [GHCR](https://github.com/cdzombak/wxcal/pkgs/container/wxcal). Images are based on the `scratch` image and are as small as possible.

A top-level directory `/ical` exists in `wxcal` containers and is the working directory for the `wxcal` tool. You can mount a volume there for easy access to generated iCal files with zero verbosity.

Run `wxcal` under Docker via, for example:

```shell
docker run --rm \
    -v /srv/ical-feeds:/ical \
    cdzombak/wxcal:1 \
    -calLocation "Ann Arbor, MI" -lat "42.27" -lon "-83.74" \
    -calDomain ics.dzombak.com \
    -v /srv/www/ical-feeds:/ical \
    -icalFile "Ann-Arbor-MI.ics"

docker run --rm \
    -v /srv/www/ical-feeds:/ical \
    ghcr.io/cdzombak/wxcal:1  \
    -calLocation "New York, NY" -lat "40.73" -lon "-73.94" \
    -calDomain ics.dzombak.com \
    -icalFile "New-York-NY.ics"
```

## About

- GitHub: [@cdzombak/wxcal](https://github.com/cdzombak/wxcal)
- [Issue tracker](https://github.com/cdzombak/wxcal/issues)
- Author: [Chris Dzombak](https://www.dzombak.com) (GitHub [@cdzombak](https://github.com/cdzombak))

## License

GNU LGPL v2.1; (c) Chris Dzombak 2019-2023. See LICENSE at the root of this repository.
