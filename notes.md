# Reworking structure of wyweb files
Changes are being made to field names, field structures, and available fields
## Renamed fields
  - location → path
  - copyright_msg → copyright
## Changed fields
  - meta is now of type []string rather than string.
## New fields
### WyWebRoot.Always
  Any values present here are ALWAYS included in the html head, and cannot be overwritten. The following fields may be
  included:
  - meta
  - copyright
  - styles
  - scripts

## Deprecated fields
## Other
  - YAML directive changed from `%YAML 1.2` to `%YAML 1.1` since `gopkg.in/yaml.v3` does not yet support all of YAML
    1.2.
