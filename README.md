# WyWeb
![](logo.svg) WyWeb is a static site generator that powers Wyatt's Websites. I originally developed the concept in 2019 and
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
A **post** is intended to be used as a blog post, or other similar type of web page. Posts are faithful to what the user
writes in that the entire document will be presented to the reader as intended. WyWeb will add navigation links, author
and version info, tags, and other metadata as necessary.

### Listings
**Listings** are directories that contain **posts**; they are rendered as a list of blog posts with an optional
thumbnail for each. Currently listings are only ordered by publication date, with the most recent post appearing at the
top of the list. A listing is generated automatically with little need for
configuration from the user.

### Galleries
My favorite of the bunch, a **gallery** is a directory of images. WyWeb will scan the directory for images,
automatically create thumbnails to save on bandwidth, and present the reader with an aesthetically pleasing grid of
images to click on.

## WyWeb Files
All types of `wyweb` files have a large overlap of settings they support, but for each type of page, there are some
settings that would not make sense to apply to other types of page. To prevent the user from having to repeat
information, WyWeb pages intelligently inherit settings from their parent directories, with the option for the user to
explicitly define exclusions if necessary.

### Common settings

In the following tables, a data type in **bold** text signifies a literal. An example would be the `prev` setting, which
tells WyWeb the previous page in a listing and has the required fields **path** and **text**:
```YAML
prev:
    path: blog/2024_02_whales
    text: Moby Dick Ruined my Garden
```

| Setting          | Type                                 | Description                                                                                                                                        | Can be inferred?                              | Heritable?        |
|------------------|--------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------|-------------------|
| author           | string                               | The author of the page                                                                                                                             | ❌                                            | ✅                |
| title            | string                               | The title (heading) of the page                                                                                                                    | ⚠  (set to contents of `<h1>` if unspecified) | ❌                |
| description      | string                               | A short description (1-2 sentences)                                                                                                                | ❌                                            | ❌                |
| copyright        | string                               | The copyright message to be displayed in the footer                                                                                                | ❌                                            | ✅                |
| date             | date                                 | The original publication date in YYYY-MM-DD format                                                                                                 | ✅                                            | ❌                |
| updated          | date                                 | The date of the most recent update to this page in YYYY-MM-DD format                                                                               | ✅                                            | ❌                |
| prev, up, next   | {**path**: string, **text**: string} | Navigation links for the previous, parent, and next pages. The **path** controls the link location, and the **text** is how the link is displayed. | ✅                                            | ❌                |
| include, exclude | list[string]                         | A list of resource names to be either included or excluded on this page                                                                            | ✅                                            | ✅                |
| meta             | list[string]                         | Intended for raw HTML `<meta>` tags, but can be any HTML To be added to the `<head>` of the document                                               | ❌                                            | ⚠ (only from root)|
| resources        | map[string:resource]                 | A map of resource names to values. See the following section                                                                                       | ❌                                            | ✅                |

#### Resources
A **resource** is a CSS Style or Javascript code. A resources can be "raw" in that their value is included directly in the page, or a "link" to be loaded separately.
| Resource Field | Type                    | Description                                                                                                                                                                                    |
|----------------|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| type           | **style** or **script** | `style` for CSS; `script` for Javascript                                                                                                                                                       |
| method         | **raw** or **link**     | determines how to treat the `value` field                                                                                                                                                      |
| attributes     | map[string:string]      | key:value pairs that render as `key="value"` in the final HTML tag                                                                                                                             |
| value          | string                  | interpreted as a URL if `method`==**link**; interpreted as code to be placed inside the `<style>` or `<script>` tags. The tags will be made automatically, so the user should not include them.|
| depends_on     | list[string]            | A list of the names of other resources that should be included before this one.                                                                                                                |

### The Root WyWeb File

The root `wyweb` must begin with the line `--- !root` (case insensitive).

