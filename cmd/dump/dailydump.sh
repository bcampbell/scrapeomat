#!/bin/bash
set -e

MDB=localhost/scotland 

# cron job to run daily
STAMP=$(date +"%Y-%m-%d")
mongo --quiet $MDB tagpublications.js >/dev/null
./dump -database $MDB scotref.db
zip scotref-$STAMP.zip scotref.db
rm scotref.db
mv scotref-$STAMP.zip /srv/vhost/steno.mediastandardstrust.org/web/
