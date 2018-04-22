DB setup:

    $ sudo -u postgres createuser --no-createdb scrape
    $ sudo -u postgres createdb -O scrape -E utf8 {DBNAME}

    $ sudoedit /etc/postgresql/9.5/main/pg_hba.conf

```
local   {DBNAME}        scrape                    peer map=scrapedev
```

/etc/postgresql/9.5/main/pg_ident.conf:
```
# MAPNAME       SYSTEM-USERNAME         PG-USERNAME
scrapedev       ben                     scrape
```

    $ sudo systemctl reload postgresql

    $ psql -U scrape {DBNAME} <store/pg/schema.sql



Scrapeomat setup




env.sh:

    $ export SCRAPEOMAT_DB="user=scrape dbname={DBNAME} host=/var/run/postgresql port=5434 sslmode=disable"


TODO:

- systemd setup
- rsyslog setup
- archive trimming (cronjob)
- nginx setup
- make sure scrapeomat has archive dir perms

