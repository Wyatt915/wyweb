# WyWeb
![](logo.svg)
WyWeb is a static site generator that powers Wyatt's Websites. I originally developed the concept in 2019 and
implemented it in Python. Eventually, the lack of a robust type system drove me mad enough to learn Go and rewrite WyWeb
from scratch. The switch to Go resulted not only in easier development, but a speed increase of nearly 1000x - from
hundreds of milliseconds to serve a page using python to *tens* of *micro*seconds using Go.

## The Structure of a WyWeb Site
A WyWeb Site is a simple directory structure. Each page corresponds to a directory that must contain a markdown document
and may contain any other resources that page needs. WyWeb uses magic `wyweb` YAML files to know how and what kind of page to
serve. Here is an example excerpt from [my website](https://wyatts.xyz):
```text
/path/to/wyatts.xyz
├── wyweb
├── blog
│   ├── 2024-01-04_uwsgi
│   │   ├── article.md
│   │   ├── logo_uWSGI.svg
│   │   └── wyweb
│   ├── 2024-01-09_derivatives
│   │   ├── article.md
│   │   └── wyweb
│   ├── 2403_led
│   │   ├── article.md
│   │   ├── reverse_led.svg
│   │   └── wyweb
│   └── wyweb
├── contact
│   ├── contact.md
│   ├── cws-pubkey.txt
│   ├── meta.json
│   └── wyweb
└── gallery
    ├── Artists drive earlier.jpg
    ├── BrycePano.jpg
    ├── cicada.jpg
    └── wyweb
```
There are three main types of WyWeb pages (with more planned for the future) — **posts**, **listings**, and
**galleries**. Each of these page types has an associated `wyweb` file to describe it. As well as these three, there is
also the **Root** `wyweb` file that controls the configuration options for the site as a whole.

### Posts

A **post** is intended to be used as a blog post, or other similar type of web page. **Posts** are faithful to what the user
writes in that the entire document will be presented to the reader as intended. WyWeb will add navigation links, author
and version info, tags, and other metadata as necessary.

### Listings

**Listings** are directories that contain **posts**

## WyWeb Files



### The Root WyWeb File

The root `wyweb` must begin with the line `--- !root` (case insensitive).

