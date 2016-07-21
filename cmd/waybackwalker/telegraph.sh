#!/bin/bash
set -e

# example for grabbing backfill links from the telegraph:


SECTIONS="/news/ /sport/ /business/ /money/ /opinion/ /travel/ /science-technology/ /culture/ /films/ /tv/ /lifestyle/ /fashion/ /luxury/ /cars/"
./waybackwalker -from 2016-03-30 -to 2016-06-13 http://www.telegraph.co.uk/ $SECTIONS
#./waybackwalker -from 2016-04-25 -to 2016-06-13 http://www.telegraph.co.uk/ $SECTIONS


