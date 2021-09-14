# wxcal

wxcal generates an iCal feed from weather.gov forecast data for a given location. The resulting feed has an all-day event for today and for each of the following 6 days; each event contains a summary of the forecast for that day along with the day's sunrise & sunset times.

For an example feed generated with this tool, see [dzombak.com/local/wxcal/Ann-Arbor-MI.ics](https://www.dzombak.com/local/wxcal/Ann-Arbor-MI.ics).

## Installation

Clone the repo (`https://github.com/cdzombak/wxcal.git`) and change into the wxcal source directory.

Run `make install` to install `wxcal` to `/usr/local/bin`.

To install somewhere else, run `make build` and move `./out/wxcal` to wherever you'd like.

### Uninstallation

Just remove the `wxcal` binary wherever it's installed. If you installed to `/usr/local/bin` with `make install`, run `make uninstall` to remove it.

## Usage

```
wxcal [-flag value] [...]

  -calDomain string
    	The calendar's domain (eg. "ical.dzombak.com") (required)
  -calLocation string
    	The name of the calendar's location (eg. "Ann Arbor, MI") (required)
  -evtTitlePrefix string
    	An optional prefix to be inserted before each event's title
  -icalFile string
    	Path/filename for iCal output file (required)
  -lat float
    	The forecast location's latitude (eg. "42.27") (default 42.27)
  -lon float
    	The forecast location's longitude (eg. "-83.74") (default -83.74)
```

Additionally, `wxcal -version` will print the version number and exit.

### Example

This invocation, run periodically via cron, generates the example feed mentioned above:

```
wxcal -calDomain ics.dzombak.com -calLocation "Ann Arbor, MI" -icalFile "/home/cdzombak/wxcal/public/Ann-Arbor-MI.ics" -lat 42.27 -lon -83.74 -evtTitlePrefix "[A2]"
```

## About

- Issues: https://github.com/cdzombak/wxcal/issues/new
- Author: [Chris Dzombak](https://www.dzombak.com)

## License

GNU LGPL v2.1; (c) Chris Dzombak 2019-2020. See LICENSE at the root of this repository.
