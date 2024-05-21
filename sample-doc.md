# This is my Markdown Document
It has all sorts of things
## Lists

  - like
  - unordered
  - lists

and also
  1. ordered
  2. lists
  3. wow!

## Code Snippets
In various languages

### Python
```python
def mu_func(n: int) -> list[int]:
    return [x for x in range(n)]
```

### C
```c
void wolfram(uint8_t* world, const uint8_t rule)
{
    uint8_t* next = malloc(WIDTH * sizeof(uint8_t));
    size_t l,c,r;
    size_t lidx, ridx;
    uint8_t current;
    for (int i = 0; i < WIDTH; i++) {
        lidx = i > 0 ? i - 1 : WIDTH - 1;
        ridx = (i + 1) % WIDTH;
        l = world[lidx];
        c = world[i];
        r = world[ridx];
        current = (l<<2) | (c<<1) | r;
        next[i] = (rule>>current) & 0b1;
    }

    for (int i = 0; i < WIDTH; i++) {
        world[i] = next[i];
    }
    free(next);
}
```

## Headings
### Can
#### Have
##### Six
###### Levels

## Tables

| This      |  Is                 |  a  Table    |
|-----------|---------------------|--------------|
| Python    |  Dynamically typed  | Interpreted  |
| Go        |  Statically Typed   | Compiled     |

<table>
<thead>
<tr>
<th>This</th><th>Is</th><th>a  Table</th>
</tr>
</thead>
<tbody>
<tr>
<td>Python</td><td>Dynamically typed</td><td>Interpreted</td>
</tr>
<tr>
<td>Go</td><td>Statically Typed</td><td>Compiled</td>
</tr>
</tbody>
</table>

## Links and images

Here is a [link](http://wyweb.site) in a paragraph.
[This link](-----------) has an invalid url.

Here is an image ![image](wyweb.png "My image")

Here is an image ![The filename is wrong!](thishasnofileext)

Here is a video ![video](bbb.webm)

Here is some audio ![audio](wyweb.mp3)

## Text Elements
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer feugiat nunc at nulla tempor tristique. Vestibulum ante
ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Fusce at feugiat orci. Duis non arcu ut nibh
semper varius sed sed felis. Aliquam viverra sem eget ante ornare egestas eu quis nisi. Mauris laoreet vitae tortor eu
consequat. Integer quis consectetur nulla. Sed tincidunt sed nisl ac accumsan.

Duis venenatis semper magna sit amet malesuada. Donec pulvinar ligula nibh, eget iaculis dolor egestas vitae. Praesent
sed tristique risus. Etiam iaculis id risus vitae tempor. Ut lobortis enim non diam aliquam dictum non at lectus. Nulla
mattis at dui ac sollicitudin. Aliquam iaculis ipsum sed ornare viverra. Aenean auctor diam non eleifend aliquam.
Integer ac nisl dolor.

Vestibulum ac nunc aliquam, facilisis metus eu, lacinia erat. Sed vel accumsan leo. Aliquam eget nulla ante. Integer
convallis dolor quis nisi mattis, at faucibus arcu suscipit. Integer et diam nibh. Duis et enim et mauris rhoncus
dictum. Nam ut nunc fermentum, finibus arcu in, sollicitudin nulla. Maecenas eleifend tristique suscipit. 

> Integer finibus arcu nec urna pulvinar, non egestas ipsum feugiat. Aliquam erat volutpat. Etiam mollis dignissim nunc
> ut cursus. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Aenean
> eleifend ante et urna tincidunt lacinia quis vitae dui. Sed iaculis mi id eros rutrum, sit amet luctus elit mollis.
> Lorem ipsum dolor sit amet, consectetur adipiscing elit. Aliquam imperdiet eleifend odio, eu feugiat quam consequat
> vel. Praesent convallis non neque a facilisis. 
