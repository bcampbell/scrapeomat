#/bin/bash
set -e

# Grab a random sample of article .warc files from arc

#ARC=/srv/jl_inna_box/arc
ARC=/home/ben/scotref/scrape/archive

pushd $ARC >/dev/null
DIRS=$(ls .)


SCRATCH=$(mktemp -d)
echo "output in $SCRATCH"

# grab 50 from each publication
for DIR in $DIRS; do
    echo $DIR
    FILES=$(find $DIR -type f | shuf -n 50)
    cp --parents $FILES $SCRATCH/
done

popd >/dev/null

echo "output in $SCRATCH"

