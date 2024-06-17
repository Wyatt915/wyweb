# Reworking structure of wyweb files
Changes are being made to field names, field structures, and available fields
## Renamed fields
  - location → path
  - copyright_msg → copyright
  - first_p → preview
## Changed fields
  - meta is now of type []string rather than string.
  - `include` is now of type []string and holds names of `resources`
  - `exclude` is now of type []string and holds names of `resources`
## New fields
### WyWebRoot.Always
  Any values present here are ALWAYS included in the html head, and cannot be overwritten. The following fields may be
  included:
  - meta
  - copyright
  - styles
  - scripts
### resources
  This is a replacement for the separate `styles` and `scripts` fields. A `resource` has the following structure
  ```go
  type Resource struct {
      Type   string `yaml:"type,omitempty"`   
      Method string `yaml:"method,omitempty"` 
      Value  string `yaml:"value,omitempty"`  
  }
  ```

## Deprecated fields
  - styles
    - Replaced by the `resources` field
  - scripts
    - Replaced by the `resources` field
## Other
  - YAML directive changed from `%YAML 1.2` to `%YAML 1.1` since `gopkg.in/yaml.v3` does not yet support all of YAML
    1.2.
