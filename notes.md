# Reworking structure of wyweb files
Changes are being made to field names, field structures, and available fields
## Renamed fields
  - location → path
  - copyright_msg → copyright
## New fields
## Deprecated fields
## Other
  - YAML directive changed from `%YAML 1.2` to `%YAML 1.1` since `gopkg.in/yaml.v3` does not yet support all of YAML
    1.2.
