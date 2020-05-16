# linkgrabber

Noddy little tool to download pages and output all the links on them.

By default grabs all links, but you can give a css selector to narrow them
down.

For example, grabbing the links in the nytimes main navigation bar:
```
$ linkgrabber -s '[data-testid="mini-nav"] a' https://nytimes.com | sort | uniq
https://nytimes.com/
https://www.nytimes.com/section/arts
https://www.nytimes.com/section/books
https://www.nytimes.com/section/business
https://www.nytimes.com/section/food
https://www.nytimes.com/section/health
https://www.nytimes.com/section/magazine
https://www.nytimes.com/section/nyregion
https://www.nytimes.com/section/opinion
https://www.nytimes.com/section/politics
https://www.nytimes.com/section/realestate
https://www.nytimes.com/section/science
https://www.nytimes.com/section/sports
https://www.nytimes.com/section/style
https://www.nytimes.com/section/technology
https://www.nytimes.com/section/t-magazine
https://www.nytimes.com/section/travel
https://www.nytimes.com/section/us
https://www.nytimes.com/section/world
https://www.nytimes.com/video
```

